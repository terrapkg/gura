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
var pool = make(chan db.Stream, 100)
var tokens []Token
var ql_current_token *Token
var qlcli = ql.NewClient("https://api.github.com/graphql", http.DefaultClient).WithRequestModifier(
	func(r *http.Request) {
		r.Header.Add("Authorization", "Bearer"+ql_current_token.key)
	})

type Token struct {
	key   string
	quota int16
	reset time.Time
	qlquo int16
	qlrst time.Time
}

func fetchGitHub(stream db.Stream) {
	pool <- stream
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
	ql_current_token = &tokens[0]
	return &tokens[0]
}

func noMoreFish(token Token) bool {
	return token.quota == 0
}
func thanksForAllTheFish(token *Token, token_idx *int) {
	*token_idx = (*token_idx + 1) % len(tokens)
	*token = tokens[*token_idx]
	ql_current_token = token
}
func waitForFish(token Token) {
	if token.quota == 0 {
		time.Sleep(time.Until(token.reset))
	}
}

func quota_reset_from_header(h http.Header) (quota int16, reset time.Time) {
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
	quota = int16(q)
	reset = time.Unix(r, 0)
	return
}
func updToken(token *Token, h http.Header) {
	token.quota, token.reset = quota_reset_from_header(h)
}
func updTokenQL(token *Token, h http.Header) {
	token.qlquo, token.qlrst = quota_reset_from_header(h)
}

// the shark shall initialise the funny
func swimGitHub() {
	token_idx := 0
	for token := fillGitHubTokens(); noMoreFish(*token); thanksForAllTheFish(token, &token_idx) {
		waitForFish(*token)
		for {
			stream := <-pool
			go func(stream db.Stream) {
				fetch(&stream, token)
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

type GHFetchType = int8

const (
	TAG GHFetchType = iota
	RELEASE
	QL
)

func fetch(stream *db.Stream, token *Token) {
	s, remain := util.SplitOnce(stream.Fetch, ' ')
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	// TODO: support other GHFetchType
	switch GHFetchType(i) {
	case TAG:
		ql_tag(stream, remain)
	case RELEASE:
		rest_release(stream, remain, token)
	}
	stream.LastChk = time.Now()
}

func ql_tag(stream *db.Stream, remain string) {
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
	updTokenQL(ql_current_token, headers)
}
