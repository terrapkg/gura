// nobori: upstream metadata fetching from various streams
//
// This file `github.go` contains routines for interacting with GitHub's REST and GraphQL APIs,
// including token management, rate limiting, and stream scheduling.
package nobori

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	ql "github.com/hasura/go-graphql-client"
	"github.com/mdobak/go-xerrors"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
)

// ? https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#about-secondary-rate-limits
// no more than 100 parallel requests
var ghRtPool = make(chan db.Stream)
var ghQlPool = make(chan db.Stream)
var ghTokRs []GHToken // GitHub REST Tokens
var ghTokQl []GHToken // GitHub GraphQL Tokens
var ghl = util.SetupLog("github")

// GitHub API authentication token
type GHToken struct {
	key   string
	quota int16
	reset time.Time
}

// Route stream to the appropriate GitHub fetch pool (REST or GraphQL) based on its type
func GhFetch(stream db.Stream) {
	t, _ := util.SplitOnce(stream.Fetch, ' ')
	i, err := strconv.ParseUint(t, 10, 8)
	util.MaybeSuicide(ghl, "bad fetch", err, zap.String("stream.Fetch", stream.Fetch))
	switch GHFetchType(i) {
	case TAG, QL:
		ghQlPool <- stream
	case RELEASE:
		ghRtPool <- stream
	}
}

// Helper to initialize a GitHub token pool by querying rate limits for each token
//
// The fetchFunc should populate quota and reset for each token.
func ghInitTokenPool(envVar string, pool *[]GHToken, fetchFunc func(token string) (GHToken, error)) *GHToken {
	token_chan := make(chan *GHToken)
	num := 0
	for token := range strings.SplitSeq(os.Getenv(envVar), ";") {
		num++
		go func(token string) {
			tok, err := fetchFunc(token)
			if err == nil {
				token_chan <- &tok
			} else {
				token_chan <- nil
			}
		}(token)
	}
	for ; num != 0; num-- {
		tok := <-token_chan
		if tok != nil {
			*pool = append(*pool, *tok)
		}
	}
	slices.SortFunc(*pool, func(a, b GHToken) int {
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
	if len(*pool) == 0 {
		ghl.Panic("no tokens from " + envVar)
	}
	l.Info("pool initialised", zap.String("envVar", envVar), zap.Int("len", len(*pool)))
	return &(*pool)[0]
}

// Initialize the REST API token pool by querying rate limits for each token
//
// Return the token with the highest quota.
func ghFillTokRt() *GHToken {
	return ghInitTokenPool("GURA_GITHUB_TOKENS", &ghTokRs, func(token string) (GHToken, error) {
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/rate_limit", nil)
		util.MaybeSuicide(ghl, "can't swim new req", err)
		req.Header.Add("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return GHToken{}, xerrors.Newf("GET /rate_limit -> give up token %s: %w", token, err)
		}
		t := GHToken{key: token}
		ghUpdTok(&t, resp.Header)
		return t, nil
	})
}

// Initialize the GraphQL API token pool by querying rate limits for each token
//
// Return the token with the highest quota.
func ghFillTokQl() *GHToken {
	return ghInitTokenPool("GURA_GITHUB_TOKENS", &ghTokQl, func(token string) (GHToken, error) {
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
		err := qlcli.Query(context.Background(), &q, nil)
		if err != nil {
			return GHToken{}, xerrors.Newf("query RateLimit -> give up token %s: %w", token, err)
		}
		reset, err := time.Parse(time.RFC3339, q.RateLimit.resetAt)
		util.MaybeSuicide(ghl, "parse time", err, zap.String("resetAt", q.RateLimit.resetAt))
		return GHToken{
			key:   token,
			quota: q.RateLimit.remaining,
			reset: reset,
		}, nil
	})
}

// Check if the token's quota is exhausted
func ghNoMoreFish(token GHToken) bool {
	return token.quota == 0
}

// Rotate to the next token in the pool
func ghThanksForAllTheFish(token *GHToken, token_idx *int, tokens *[]GHToken) {
	*token_idx = (*token_idx + 1) % len(*tokens)
	*token = (*tokens)[*token_idx]
}

// Sleep until the token's quota reset time if quota is exhausted
func ghWaitForFish(token GHToken) {
	if token.quota == 0 {
		ghl.Info("wait for token reset", zap.Time("reset", token.reset))
		time.Sleep(time.Until(token.reset))
	}
}

// Update token quota and reset time from HTTP response headers
func ghUpdTok(token *GHToken, h http.Header) {
	q, err := strconv.ParseInt(h.Get("x-ratelimit-remaining"), 10, 16)
	util.MaybeSuicide(ghl, "strconv x-ratelimit-remaining", err)
	r, err := strconv.ParseInt(h.Get("x-ratelimit-reset"), 10, 64)
	util.MaybeSuicide(ghl, "strconv x-ratelimit-reset", err)
	token.quota = int16(q)
	token.reset = time.Unix(r, 0)
}

// the shark shall initialise the funny
func GhSwim() {
	go ghSwimRt()
	go ghSwimQl()
}

// Process GitHub REST streams using available tokens
func ghSwimRt() {
	for i, token := 0, ghFillTokRt(); ghNoMoreFish(*token); ghThanksForAllTheFish(token, &i, &ghTokRs) {
		ghWaitForFish(*token)
		for !ghNoMoreFish(*token) {
			stream := <-ghRtPool
			ghFetch(&stream, token, nil)
			go schedule(stream)
			go db.DB.Save(stream)
		}
		ghl.Info("ran out of fish", zap.Int("token_idx", i))
	}
}

// Process GitHub GraphQL streams using available tokens
func ghSwimQl() {
	for i, token := 0, ghFillTokQl(); ghNoMoreFish(*token); ghThanksForAllTheFish(token, &i, &ghTokQl) {
		ghWaitForFish(*token)
		qlcli := ql.NewClient("https://api.github.com/graphql", http.DefaultClient).WithRequestModifier(
			func(r *http.Request) {
				r.Header.Add("Authorization", "Bearer "+token.key)
			})
		for {
			stream := <-ghQlPool
			go func(stream db.Stream) {
				ghFetch(&stream, token, qlcli)
				go schedule(stream)
				db.DB.Save(stream)
			}(stream)
		}
	}
}

// Fetch latest GitHub release using the REST API
//
// Update the stream's version if a new release is found.
func ghRtRelease(stream *db.Stream, remain string, token *GHToken) {
	prefix, remain := util.SplitOnce(remain, ' ')
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+remain+"/releases", nil)
	if util.Yeet(ghl, "req fail", err, zap.String("fetch", stream.Fetch)) {
		return
	}
	req.Header.Add("Authorization", "Bearer "+token.key)
	resp, err := http.DefaultClient.Do(req)
	if util.Yeet(ghl, "resp fail", err, zap.String("fetch", stream.Fetch)) {
		return
	}
	ghUpdTok(token, resp.Header)
	buf, err := io.ReadAll(resp.Body)
	util.MaybeSuicide(ghl, "can't read buf", err)
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
	ghl.Warn("can't find prefix", zap.String("fetch", stream.Fetch))
}

// Supported GitHub fetch modes
type GHFetchType = uint8

const (
	TAG GHFetchType = iota
	RELEASE
	QL
)

// Dispatch a stream to the appropriate ghFetch handler based on its type and ghFetch mode
func ghFetch(stream *db.Stream, token *GHToken, qlcli *ql.Client) {
	s, remain := util.SplitOnce(stream.Fetch, ' ')
	i, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		panic(err)
	}
	switch GHFetchType(i) {
	case TAG:
		ghQlTag(stream, remain, token, *qlcli)
	case RELEASE:
		ghRtRelease(stream, remain, token)
	case QL:
		panic("todo") // TODO: ql fetch mode
	}
	stream.LastChk = time.Now()
}

// Fetch the latest tag from a GitHub repository using the GraphQL API
//
// Update the stream's version if a new tag is found.
func ghQlTag(stream *db.Stream, remain string, token *GHToken, qlcli ql.Client) {
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
	if util.Yeet(ghl, "ql_tag", qlcli.Query(context.Background(), &q, map[string]any{
		"prefix": prefix,
		"owner":  owner,
		"name":   name,
	}, ql.BindResponseHeaders(&headers))) {
		return
	}
	if v := strings.TrimPrefix(q.Repository.Refs.edges[0].node.name, prefix); v != stream.Ver {
		stream.Ver = v
		stream.LastUpd = time.Now()
	}
	ghUpdTok(token, headers)
}
