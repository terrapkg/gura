/*
gura

Copyright (c) 2024-2025 Fyra Labs

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// nobori: upstream metadata fetching from various streams
//
// This file `github.go` contains routines for interacting with GitHub's REST and GraphQL APIs,
// including token management, rate limiting, and stream scheduling.
//
// # Why REST & GraphQL
//
// The REST API currently does not support sorting tags by their date of creation; and it currently
// sorts them alphabetically, making it harder to obtain the latest tag.
//
// The GraphQL API on the other hand allows sorting by the date of commit for tags.
//
// Additionally GraphQL and REST provide separate primary rate limits (each 5000 points), meaning
// more streams may be fetched with less tokens!
package nobori

import (
	"context"
	"encoding/json"
	"fmt"
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

const GH_WARN_DUR = 500 // ms

// ? https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#about-secondary-rate-limits
// no more than 100 parallel requests
var ghRtPool = make(chan db.Stream, 100)
var ghQlPool = make(chan db.Stream, 100)
var ghTokRt []GHToken // GitHub REST Tokens
var ghTokQl []GHToken // GitHub GraphQL Tokens
var ghRtTokIdx int = 0
var ghQlTokIdx int = 0
var ghl = util.SetupLog("github")

// ————————————————————————————————————————————————————————————————————————————
// Tokens

// GitHub API authentication token
type GHToken struct {
	key   string
	quota int16
	reset time.Time
}

// Create a new GraphQL client
func (token *GHToken) qlcli() *ql.Client {
	return ql.NewClient("https://api.github.com/graphql", http.DefaultClient).WithRequestModifier(
		func(r *http.Request) {
			r.Header.Add("Authorization", "Bearer "+token.key)
		})
}

// Check if the token's quota is exhausted
func (token *GHToken) noMoreFish() bool {
	return token.quota == 0
}

// Rotate to the next token in the pool
func (token *GHToken) thanksForAllTheFish(tokenIdx *int, tokens *[]GHToken) {
	*tokenIdx = (*tokenIdx + 1) % len(*tokens)
	*token = (*tokens)[*tokenIdx]
}

// Sleep until the token's quota reset time if quota is exhausted
func (token *GHToken) waitForFish() {
	if token.quota == 0 {
		ghl.Info("wait for token reset", zap.Time("reset", token.reset))
		time.Sleep(time.Until(token.reset))
	}
}

// Update token quota and reset time from HTTP response headers
func (token *GHToken) updTok(h http.Header) {
	q, err := strconv.ParseInt(h.Get("x-ratelimit-remaining"), 10, 16)
	util.MaybeSuicide(ghl, "strconv", err, zap.String("x-ratelimit-remaining", h.Get("x-ratelimit-remaining")))
	r, err := strconv.ParseInt(h.Get("x-ratelimit-reset"), 10, 64)
	util.MaybeSuicide(ghl, "strconv", err, zap.String("x-ratelimit-reset", h.Get("x-ratelimit-reset")))
	token.quota = int16(q)
	token.reset = time.Unix(r, 0)
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
func ghInitTokenPool(pool *[]GHToken, fetchFunc func(token string) (GHToken, error)) *GHToken {
	token_chan := make(chan *GHToken)
	num := 0
	for token := range strings.SplitSeq(os.Getenv("GURA_GITHUB_TOKENS"), ";") {
		if token == "" {
			continue
		}
		num++
		go func(token string) {
			tok, err := fetchFunc(token)
			if err == nil {
				token_chan <- &tok
			} else {
				util.Yeet(l, "cannot fetch", err)
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
	if len(*pool) == 0 {
		ghl.Panic("no tokens from GURA_GITHUB_TOKENS")
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
	l.Info("pool initialised", zap.Int("len", len(*pool)))
	return &(*pool)[0]
}

// Initialize the REST API token pool by querying rate limits for each token
//
// Return the token with the highest quota.
func ghFillTokRt() *GHToken {
	return ghInitTokenPool(&ghTokRt, func(token string) (GHToken, error) {
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/rate_limit", nil)
		util.MaybeSuicide(ghl, "can't swim new req", err)
		req.Header.Add("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return GHToken{}, xerrors.Newf("GET /rate_limit -> give up token %s: %w", token, err)
		}
		t := GHToken{key: token}
		t.updTok(resp.Header)
		return t, nil
	})
}

// Initialize the GraphQL API token pool by querying rate limits for each token
//
// Return the token with the highest quota.
func ghFillTokQl() *GHToken {
	return ghInitTokenPool(&ghTokQl, func(token string) (tok GHToken, err error) {
		tok.key = token
		qlcli := tok.qlcli()
		var q struct {
			RateLimit struct {
				Remaining int16
				ResetAt   string
			}
		}
		err = qlcli.Query(context.Background(), &q, nil)
		if err != nil {
			return GHToken{}, xerrors.Newf("query RateLimit -> give up token %s: %w", token, err)
		}
		reset, err := time.Parse(time.RFC3339, q.RateLimit.ResetAt)
		util.MaybeSuicide(ghl, "parse time", err, zap.String("resetAt", q.RateLimit.ResetAt))
		return GHToken{
			key:   token,
			quota: q.RateLimit.Remaining,
			reset: reset,
		}, nil
	})
}

// ————————————————————————————————————————————————————————————————————————————
// Swimming

// the shark shall initialise the funny
func GhSwim() chan struct{} {
	go ghSwimRt()
	go ghSwimQl()
	ready := make(chan struct{}, 1)
	go func() {
		for len(ghTokRt) == 0 || len(ghTokQl) == 0 {
			time.Sleep(1 * time.Millisecond)
		}
		ready <- struct{}{}
	}()
	return ready
}

// Process GitHub REST streams using available tokens
func ghSwimRt() {
	for token := ghFillTokRt(); ; token.thanksForAllTheFish(&ghRtTokIdx, &ghTokRt) {
		token.waitForFish()
		for !token.noMoreFish() {
			stream := <-ghRtPool
			ghFetch(&stream, token, nil)
			go schedule(stream)
			go db.DB.Save(stream)
		}
		ghl.Info("ran out of fish", zap.Int("token_idx", ghRtTokIdx))
	}
}

// Process GitHub GraphQL streams using available tokens
func ghSwimQl() {
	for token := ghFillTokQl(); ; token.thanksForAllTheFish(&ghQlTokIdx, &ghTokQl) {
		token.waitForFish()
		qlcli := token.qlcli()
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

// ————————————————————————————————————————————————————————————————————————————
// Fetching

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

func ghRtReleaseCall(stream *db.Stream, repo string, token *GHToken) []string {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repo+"/releases", nil)
	if util.Yeet(ghl, "req fail", err, zap.String("fetch", stream.Fetch)) {
		return []string{}
	}
	req.Header.Add("Authorization", "Bearer "+token.key)
	l.Debug("ghRtReleaseCall", zap.String("repo", repo))
	t0 := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if t := time.Since(t0); t.Milliseconds() > GH_WARN_DUR {
		l.Warn("took " + t.String())
	}
	if util.Yeet(ghl, "resp fail", err, zap.String("fetch", stream.Fetch)) {
		return []string{}
	}
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			l.Error("can't read body", zap.String("fetch", stream.Fetch), zap.Error(err))
			body = []byte{}
		}
		l.Error("bad status", zap.String("status", resp.Status), zap.String("fetch", stream.Fetch), zap.ByteString("body", body))
		return []string{}
	}
	token.updTok(resp.Header)
	buf, err := io.ReadAll(resp.Body)
	util.MaybeSuicide(ghl, "can't read buf", err)
	type Resp struct {
		Tag string `json:"tag_name"`
	}

	var v []Resp
	if util.Yeet(l, "can't unmarshal", json.Unmarshal(buf, &v), zap.ByteString("resp", buf)) {
		return []string{}
	}
	return util.SliceMap(v, func(r Resp) string { return r.Tag })
}

// Fetch latest GitHub release using the REST API
//
// Update the stream's version if a new release is found.
func ghRtRelease(stream *db.Stream, remain string, token *GHToken) {
	prefix, repo := util.SplitOnce(remain, ' ')
	releases := ghRtReleaseCall(stream, repo, token)
	if len(releases) == 0 {
		return
	}

	for _, v := range releases {
		if v, ok := strings.CutPrefix(v, prefix); ok {
			if stream.Ver != v {
				stream.Ver = v
				stream.LastUpd = time.Now()
			}
			return
		}
	}
	ghl.Warn("can't find prefix", zap.String("fetch", stream.Fetch))
}

// Fetch the latest tag from a GitHub repository using the GraphQL API
//
// Update the stream's version if a new tag is found.
func ghQlTagCall(prefix, owner, name string, len int, qlcli ql.Client, token *GHToken) []string {
	var q struct {
		Repository struct { // https://docs.github.com/en/graphql/reference/objects#repository
			Refs struct { // https://docs.github.com/en/graphql/reference/objects#refconnection
				Edges []struct {
					Node struct {
						Name string
					}
				}
			} `graphql:"refs(refPrefix: \"refs/tags/\", last: $len, orderBy: {field: TAG_COMMIT_DATE, direction: ASC}, query: $prefix)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	headers := http.Header{}
	l.Debug("ghQlTagCall", zap.String("repo", owner+"/"+name))
	t0 := time.Now()
	err := qlcli.Query(context.TODO(), &q, map[string]any{
		"prefix": prefix,
		"owner":  owner,
		"name":   name,
		"len":    len,
	}, ql.BindResponseHeaders(&headers))
	if t := time.Since(t0); t.Milliseconds() > GH_WARN_DUR {
		l.Warn("took " + t.String())
	}
	if util.Yeet(ghl, "ql_tag_call", err) {
		return nil
	}

	tags := make([]string, 0, len)
	for _, edge := range q.Repository.Refs.Edges {
		tags = append(tags, edge.Node.Name)
	}
	token.updTok(headers)
	return tags
}

// Fetch the latest tag from a GitHub repository using the GraphQL API
// Update the stream's version if a new tag is found.
func ghQlTag(stream *db.Stream, remain string, token *GHToken, qlcli ql.Client) {
	prefix, remain := util.SplitOnce(remain, ' ')
	owner, name := util.SplitOnce(remain, '/')
	tags := ghQlTagCall(prefix, owner, name, 1, qlcli, token)
	if len(tags) == 0 {
		return
	}
	if v := strings.TrimPrefix(tags[0], prefix); v != stream.Ver {
		stream.Ver = v
		stream.LastUpd = time.Now()
	}
}

const GH_STRM_HDLR_TEST_QL_MAX = 20

func GhStrmHdlr(pkg db.Pkg, url string) *db.Stream {
	strm := &db.Stream{
		Forge: db.GitHub,
	}
	repo := strings.TrimSuffix(strings.TrimPrefix(url, "github.com/"), "/")
	tokenrt := &ghTokRt[ghRtTokIdx]
	if tokenrt.noMoreFish() {
		tokenrt.waitForFish()
	}

	// GHFetchType: RELEASE
	releases := ghRtReleaseCall(strm, repo, tokenrt)
	if len(releases) != 0 {
		for _, release := range releases {
			if prefix, found := strings.CutSuffix(release, pkg.Ver); found {
				strm.Fetch = fmt.Sprintf("%d %s %s", RELEASE, prefix, repo)
				return strm
			}
		}
		l.Warn("Found releases, but cannot determine ver/prefix",
			zap.Strings("releases", releases),
			zap.String("repo", repo),
			zap.String("pkgid", pkg.ID.String()),
			zap.String("pkgv", pkg.Ver))
	}

	tokenql := &ghTokQl[ghQlTokIdx]
	if tokenql.noMoreFish() {
		tokenql.waitForFish()
	}
	qlcli := tokenql.qlcli()

	// GHFetchType: TAG
	owner, name := util.SplitOnce(repo, '/')
	tags := ghQlTagCall("", owner, name, GH_STRM_HDLR_TEST_QL_MAX, *qlcli, tokenql)
	if len(tags) > 0 {
		for _, tag := range tags {
			if prefix, found := strings.CutSuffix(tag, pkg.Ver); found {
				strm.Fetch = fmt.Sprintf("%d %s %s", TAG, prefix, repo)
				return strm
			}
		}
		l.Warn("Found tags, but cannot determine ver/prefix",
			zap.Strings("tags", tags),
			zap.String("repo", repo),
			zap.String("pkgid", pkg.ID.String()))
	}

	// GHFetchType: QL
	// TODO

	l.Warn("No available methods for determining ver",
		zap.String("repo", repo),
		zap.String("pkgid", pkg.ID.String()))
	return nil
}
