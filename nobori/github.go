package nobori

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	ql "github.com/hasura/go-graphql-client"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/util"
)

// ? https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#about-secondary-rate-limits
// no more than 100 parallel requests
var pool_rest = make(chan db.Stream)
var pool_ql = make(chan db.Stream)
var tokens []Token
var tokensql []Token

type Token struct {
	key   string
	quota int16
	reset time.Time
}

func fetchGitHub(stream db.Stream) {
	t, _ := util.SplitOnce(stream.Fetch, ' ')
	i, err := strconv.ParseUint(t, 10, 8)
	if err != nil {
		log.Fatalf("github: bad fetch [%s]: %v", stream.Fetch, err)
	}
	switch GHFetchType(i) {
	case TAG, QL:
		pool_ql <- stream
	case RELEASE:
		pool_rest <- stream
	}
}

func fillGitHubTokens() *Token {
	token_chan := make(chan Token)
	num := 0
	for token := range strings.SplitSeq(os.Getenv("GURA_GITHUB_TOKENS"), ";") {
		num++
		go func(token string) {
			req, err := http.NewRequest(http.MethodGet, "https://api.github.com/rate_limit", nil)
			if err != nil {
				log.Fatalln("github: can't swim new req:", err)
			}
			req.Header.Add("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("github: GET /rate_limit: %v\n"+
					"github: give up on token: %s", err, token)
				return
			}
			quota, err := strconv.ParseInt(resp.Header.Get("x-ratelimit-remaining"), 10, 16)
			if err != nil {
				log.Printf("github: strconv x-ratelimit-remaining: %v\n"+
					"github: give up on token: %s", err, token)
				return
			}
			reset, err := strconv.ParseInt(resp.Header.Get("x-ratelimit-reset"), 10, 64)
			if err != nil {
				log.Printf("github: strconv x-ratelimit-reset: %v\n"+
					"github: give up on token: %s", err, token)
				return
			}
			token_chan <- Token{
				key:   token,
				quota: int16(quota),
				reset: time.Unix(reset, 0),
			}
		}(token)
	}
	for ; num != 0; num-- {
		tokens = append(tokens, <-token_chan)
	}
	slices.SortFunc(tokens, func(a, b Token) int {
		if a.quota == 0 && b.quota == 0 {
			return a.reset.Compare(b.reset)
		}
		if a.quota < b.quota {
			return -1
		}
		if a.quota == b.quota {
			return 0
		}
		return +1
	})
	if len(tokens) == 0 {
		log.Fatalln("github: fatal: no tokens")
	}
	return &tokens[0]
}
func fillGitHubTokensQL() *Token {
	token_chan := make(chan Token)
	num := 0
	for token := range strings.SplitSeq(os.Getenv("GURA_GITHUB_TOKENS"), ";") {
		num++
		go func(token string) {
			var qlcli = ql.NewClient("https://api.github.com/graphql", http.DefaultClient).WithRequestModifier(
				func(r *http.Request) {
					r.Header.Add("Authorization", "Bearer "+token)
				})
			var q struct {
				RateLimit struct {
					remaining int16
					resetAt   string
				}
			}
			if err := qlcli.Query(context.Background(), &q, nil); err != nil {
				log.Fatalln("github: can't query RateLimit for token", token)
			}
			reset, err := time.Parse(time.RFC3339, q.RateLimit.resetAt)
			if err != nil {
				log.Fatalln("github: cannot parse time:", q.RateLimit.resetAt)
			}
			token_chan <- Token{
				key:   token,
				quota: q.RateLimit.remaining,
				reset: reset,
			}
		}(token)
	}
	for ; num != 0; num-- {
		tokensql = append(tokensql, <-token_chan)
	}
	slices.SortFunc(tokensql, func(a, b Token) int {
		if a.quota == 0 && b.quota == 0 {
			return a.reset.Compare(b.reset)
		}
		if a.quota < b.quota {
			return -1
		}
		if a.quota == b.quota {
			return 0
		}
		return +1
	})
	if len(tokens) == 0 {
		log.Fatalln("github: fatal: no tokens")
	}
	return &tokensql[0]
}

func noMoreFish(token Token) bool {
	return token.quota == 0
}
func thanksForAllTheFish(token *Token, token_idx *int, tokens *[]Token) {
	*token_idx = (*token_idx + 1) % len(*tokens)
	*token = (*tokens)[*token_idx]
}
func waitForFish(token Token) {
	if token.quota == 0 {
		time.Sleep(time.Until(token.reset))
	}
}
func updToken(token *Token, h http.Header) {
	q, err := strconv.ParseInt(h.Get("x-ratelimit-remaining"), 10, 16)
	if err != nil {
		log.Fatalf("github: strconv x-ratelimit-remaining: %v", err)
		return
	}
	r, err := strconv.ParseInt(h.Get("x-ratelimit-reset"), 10, 64)
	if err != nil {
		log.Fatalf("github: strconv x-ratelimit-reset: %v", err)
		return
	}
	token.quota = int16(q)
	token.reset = time.Unix(r, 0)
}

// the shark shall initialise the funny
func swimGitHub() {
	go swimGitHubRest()
	go swimGitHubQL()
}
func swimGitHubRest() {
	for i, token := 0, fillGitHubTokens(); noMoreFish(*token); thanksForAllTheFish(token, &i, &tokens) {
		waitForFish(*token)
		for {
			stream := <-pool_rest
			go func(stream db.Stream) {
				fetch(&stream, token, nil)
				go schedule(stream)
				db.DB.Save(stream)
			}(stream)
		}
	}
}
func swimGitHubQL() {
	for i, token := 0, fillGitHubTokensQL(); noMoreFish(*token); thanksForAllTheFish(token, &i, &tokensql) {
		waitForFish(*token)
		qlcli := ql.NewClient("https://api.github.com/graphql", http.DefaultClient).WithRequestModifier(
			func(r *http.Request) {
				r.Header.Add("Authorization", "Bearer "+token.key)
			})
		for {
			stream := <-pool_ql
			go func(stream db.Stream) {
				fetch(&stream, token, qlcli)
				go schedule(stream)
				db.DB.Save(stream)
			}(stream)
		}
	}
}

func rest_release(stream *db.Stream, remain string, token *Token) {
	prefix, remain := util.SplitOnce(remain, ' ')
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+remain+"/releases", nil)
	if err != nil {
		log.Printf("github: req [%s]: %v", stream.Fetch, err)
		return
	}
	req.Header.Add("Authorization", "Bearer "+token.key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("github: resp [%s]: %v", stream.Fetch, err)
		return
	}
	updToken(token, resp.Header)
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("github: can't read buf")
	}
	var v []struct {
		tag_name string
	}
	json.Unmarshal(buf, &v)
	for _, v := range v {
		if v, ok := strings.CutPrefix(v.tag_name, prefix); ok {
			if stream.Ver != v {
				stream.Ver = v
				stream.LastUpd = time.Now()
			}
			return
		}
	}
	log.Printf("github: [%s]: can't find prefix", stream.Fetch)
}

type GHFetchType = uint8

const (
	TAG GHFetchType = iota
	RELEASE
	QL
)

func fetch(stream *db.Stream, token *Token, qlcli *ql.Client) {
	s, remain := util.SplitOnce(stream.Fetch, ' ')
	i, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		panic(err)
	}
	switch GHFetchType(i) {
	case TAG:
		ql_tag(stream, remain, token, *qlcli)
	case RELEASE:
		rest_release(stream, remain, token)
	case QL:
		panic("todo") // TODO: ql fetch mode
	}
	stream.LastChk = time.Now()
}

func ql_tag(stream *db.Stream, remain string, token *Token, qlcli ql.Client) {
	prefix, remain := util.SplitOnce(remain, ' ')
	owner, name := util.SplitOnce(remain, '/')
	// ? https://stackoverflow.com/questions/19452244/github-api-v3-order-tags-by-creation-date
	var q struct {
		Repository struct {
			Refs struct {
				edges []struct {
					node struct {
						name string
					}
				}
			} `graphql:"refs(prefix: \"refs/tags/\", last: 1, orderBy: {field: TAG_COMMIT_DATE, direction: ASC}), query: $prefix"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	var headers http.Header
	if err := qlcli.Query(context.Background(), &q, map[string]any{
		"prefix": prefix,
		"owner":  owner,
		"name":   name,
	}, ql.BindResponseHeaders(&headers)); err != nil {
		log.Printf("github: ql_tag: %v", err)
		return
	}
	if v := strings.TrimPrefix(q.Repository.Refs.edges[0].node.name, prefix); v != stream.Ver {
		stream.Ver = v
		stream.LastUpd = time.Now()
	}
	updToken(token, headers)
}
