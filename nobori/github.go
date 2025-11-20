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
const GH_STRM_HDLR_TEST_QL_MAX = 20

var ghPrioPool = make(chan GHJob, 100)
var ghPool = make(chan GHJob, 100)
var ghTokmgr = GHTokMgr{}
var ghl = util.SetupLog("github")

// Fetch data from GitHub
//
// Create a new job to fetch the latest version of the stream
func GhFetch(stream db.Stream) {
	defer schedule(stream)
	job := GHJob{
		Result: make(chan string),
		Init:   false,
		Fetch:  stream.Fetch,
	}
	ghPool <- job
	ver, ok := <-job.Result
	if !ok {
		l.Error("cannot fetch stream")
		return
	}
	if ver != stream.Ver {
		stream.Ver = ver
		stream.LastUpd = time.Now()
	}
	stream.LastChk = time.Now()
	util.Yeet(ghl, "cannot save stream", db.DB.Save(stream).Error)
}

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

type GHTokMgr struct {
	rtToks               []GHToken // GitHub REST Tokens
	qlToks               []GHToken // GitHub GraphQL Tokens
	rtIdx                int
	qlIdx                int
	ghWaitSecondaryLimit time.Time // TODO: secondary rate limit handling
}

// Helper to initialize a GitHub token pool by querying rate limits for each token
//
// The fetchFunc should populate quota and reset for each token.
func (mgr *GHTokMgr) ghInitTokenPool(pool *[]GHToken, fetchFunc func(token string) (GHToken, error)) {
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
				util.Yeet(ghl, "cannot fetch", err)
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
	ghl.Info("pool initialised", zap.Int("len", len(*pool)))
}

// Initialize the REST API token pool by querying rate limits for each token
//
// Return the token with the highest quota.
func (mgr *GHTokMgr) ghFillTokRt() {
	mgr.ghInitTokenPool(&mgr.rtToks, func(token string) (GHToken, error) {
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/rate_limit", nil)
		util.MaybeSuicide(ghl, "can't swim new req", err)
		req.Header.Add("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return GHToken{}, err
		}
		var t GHToken
		t.key = token
		t.updTok(resp.Header)
		return t, nil
	})
}

// Initialize the GraphQL API token pool by querying rate limits for each token
//
// Return the token with the highest quota.
func (mgr *GHTokMgr) ghFillTokQl() {
	mgr.ghInitTokenPool(&mgr.qlToks, func(token string) (tok GHToken, err error) {
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
		if err != nil {
			return GHToken{}, err
		}
		tok.quota = q.RateLimit.Remaining
		tok.reset = reset
		return tok, nil
	})
}

// Initialize the token pools
//
// We assume there are no more than 50 tokens, such that
// the concurrent limit is not exceeded.
func (tokmgr *GHTokMgr) init() {
	rt_ok := make(chan struct{}, 1)
	go func() {
		tokmgr.ghFillTokRt()
		rt_ok <- struct{}{}
	}()
	tokmgr.ghFillTokQl()
	<-rt_ok
}

// Obtain and wait for the current GraphQL API token
func (tokmgr *GHTokMgr) ql() *GHToken {
	token := &tokmgr.qlToks[tokmgr.qlIdx]
	token.waitForFish()
	return token
}

// Obtain and wait for the current REST API token
func (tokmgr *GHTokMgr) rt() *GHToken {
	token := &tokmgr.rtToks[tokmgr.rtIdx]
	token.waitForFish()
	return token
}

// Rotate to the next GraphQL token in the pool
func (tokmgr *GHTokMgr) thanksForAllTheFishQl(token **GHToken) {
	// we must not use defer (*token).waitForFish(),
	// as (*token) is evaluated in-place.
	util.Assert((*token).noMoreFish())
	if *token != &tokmgr.qlToks[tokmgr.qlIdx] {
		*token = &tokmgr.qlToks[tokmgr.qlIdx]
		(*token).waitForFish()
		return
	}
	tokmgr.qlIdx = (tokmgr.qlIdx + 1) % len(tokmgr.qlToks)
	*token = &tokmgr.qlToks[tokmgr.qlIdx]
	(*token).waitForFish()
}

// Rotate to the next REST token in the pool
func (tokmgr *GHTokMgr) thanksForAllTheFishRt(token **GHToken) {
	util.Assert((*token).noMoreFish())
	if *token != &tokmgr.rtToks[tokmgr.rtIdx] {
		*token = &tokmgr.rtToks[tokmgr.rtIdx]
		(*token).waitForFish()
		return
	}
	tokmgr.rtIdx = (tokmgr.rtIdx + 1) % len(tokmgr.rtToks)
	*token = &tokmgr.rtToks[tokmgr.rtIdx]
	(*token).waitForFish()
}

// ————————————————————————————————————————————————————————————————————————————
// Swimming

// the shark shall initialise the funny
func GhSwimInit() chan struct{} {
	ready := make(chan struct{}, 1)
	go func() {
		ghTokmgr.init()
		ready <- struct{}{}
		go ghSwim()
	}()
	return ready
}

// Schedule jobs
//
// Run max. 100 jobs concurrently as required by GitHub.
// See the rate limit documentation for more information:
//
// https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#about-secondary-rate-limits
func ghSwim() {
	finishes := make(chan uint8, 100)
	available := make(chan uint8, 100)
	for i := range 100 {
		available <- uint8(i)
	}
	for {
		for len(available) == 0 || len(finishes) != 0 {
			available <- <-finishes
		}
		var job GHJob
		var idx uint8
		select {
		case job = <-ghPrioPool:
			idx = <-available
		case job = <-ghPool:
			idx = <-available
		default:
			continue
		}
		go job.run(idx, finishes)
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

type GHJob struct {
	Result chan string
	Init   bool
	Fetch  string
}

func (job GHJob) run(idx uint8, finishes chan uint8) {
	s, remain := util.SplitOnce(job.Fetch, ' ')
	i, err := strconv.ParseUint(s, 10, 8)
	util.Assert(err == nil)
	switch GHFetchType(i) {
	case TAG:
		job.qlTag(remain)
	case RELEASE:
		job.rtRelease(remain)
	case QL:
		panic("todo") // TODO: ql fetch mode
	}
	finishes <- idx
}

func (job GHJob) CallQl(q any, vars map[string]any) error {
	tok := ghTokmgr.ql()
retry:
	headers := http.Header{}
	t0 := time.Now()
	err := tok.qlcli().Query(context.TODO(), q, vars, ql.BindResponseHeaders(&headers))
	if t := time.Since(t0); t.Milliseconds() > GH_WARN_DUR {
		l.Warn("took " + t.String())
	}
	if err == nil {
		tok.updTok(headers)
		return nil
	}
	if len(headers) == 0 {
		return err
	}
	tok.updTok(headers)
	if tok.noMoreFish() {
		ghTokmgr.thanksForAllTheFishQl(&tok)
		goto retry
	}
	return err
}

func (job GHJob) qlTag(remain string) {
	defer close(job.Result)
	prefix, remain := util.SplitOnce(remain, ' ')
	owner, name := util.SplitOnce(remain, '/')
	length := 1
	if job.Init {
		length = GH_STRM_HDLR_TEST_QL_MAX
	}

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
	err := job.CallQl(&q, map[string]any{
		"prefix": prefix,
		"owner":  owner,
		"name":   name,
		"len":    length,
	})
	if util.Yeet(ghl, "ql_tag_call", err) {
		return
	}

	for _, edge := range q.Repository.Refs.Edges {
		job.Result <- edge.Node.Name
	}
}

func (job GHJob) CallRt(method string, url string) *http.Response {
	tok := ghTokmgr.rt()
retry:
	req, err := http.NewRequest(method, url, nil)
	if util.Yeet(ghl, "CallRt fail", xerrors.Newf("new req fail: %w", err)) {
		return nil
	}
	req.Header.Add("Authorization", "Bearer "+tok.key)
	t0 := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if t := time.Since(t0); t.Milliseconds() > GH_WARN_DUR {
		l.Warn("took " + t.String())
	}
	switch {
	case err != nil:
		l.Error("CallRt resp fail", zap.String("fetch", job.Fetch), zap.Error(err))
		return nil
	case resp.StatusCode == http.StatusOK:
		tok.updTok(resp.Header)
		return resp
	// ? https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#exceeding-the-rate-limit
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden:
		tok.updTok(resp.Header)
		if tok.noMoreFish() {
			ghTokmgr.thanksForAllTheFishRt(&tok)
			goto retry
		}
		// TODO: handle secondary rate limit
		if resp.Header.Get("retry-after") != "" {
			secs, err := strconv.ParseUint(resp.Header.Get("retry-after"), 10, 64)
			util.MaybeSuicide(ghl, "fail to parse retry-after header", err, zap.String("fetch", job.Fetch))
			// also stop other concurrent requests
			time.Sleep(time.Duration(secs) * time.Second)
			goto retry
		}
		// TODO: handle exponential backoff
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		l.Error("can't read body", zap.String("fetch", job.Fetch), zap.Error(err))
		body = []byte{}
	}
	l.Error("bad status", zap.String("status", resp.Status), zap.String("fetch", job.Fetch), zap.ByteString("body", body))
	return nil
}

func (job GHJob) rtRelease(remain string) {
	defer close(job.Result)
	prefix, repo := util.SplitOnce(remain, ' ')
	resp := job.CallRt(http.MethodGet, "https://api.github.com/repos/"+repo+"/releases")
	if resp == nil {
		return
	}

	buf, err := io.ReadAll(resp.Body)
	util.MaybeSuicide(ghl, "can't read buf", err)
	type Resp struct {
		Tag string `json:"tag_name"`
	}

	var v []Resp
	if util.Yeet(ghl, "can't unmarshal", json.Unmarshal(buf, &v), zap.ByteString("resp", buf)) {
		return
	}
	for _, r := range v {
		after, found := strings.CutPrefix(r.Tag, prefix)
		if found {
			job.Result <- after
			if job.Init {
				return
			}
		}
	}
}

func GhStrmHdlr(strm_job *StrmChkJob, url string) bool {
	strm := &db.Stream{
		Forge: db.GitHub,
	}
	repo := strings.TrimSuffix(strings.TrimPrefix(url, "github.com/"), "/")

	// GHFetchType: RELEASE
	job := GHJob{
		Result: make(chan string),
		Init:   true,
		Fetch:  fmt.Sprintf("%d %s %s", RELEASE, "", repo),
	}
	ghPrioPool <- job
	if strm_job.Manifest(func(ver string) *db.Stream {
		var releases []string
		for release := range job.Result {
			releases = append(releases, release)
			if prefix, found := strings.CutSuffix(release, ver); found {
				strm.Fetch = fmt.Sprintf("%d %s %s", RELEASE, prefix, repo)
				strm.LastChk = time.Now()
				strm.LastUpd = time.Now()
				strm.Ver = ver
				return strm
			}
		}
		if len(releases) > 0 {
			l.Warn("Found releases, but cannot determine ver/prefix",
				zap.Strings("releases", releases),
				zap.String("repo", repo),
				zap.String("ver", ver))
		}
		return nil
	}) {
		return true
	}

	// GHFetchType: TAG
	job = GHJob{
		Result: make(chan string),
		Init:   true,
		Fetch:  fmt.Sprintf("%d %s %s", TAG, "", repo),
	}
	ghPrioPool <- job

	if strm_job.Manifest(func(ver string) *db.Stream {
		var tags []string
		for tag := range job.Result {
			tags = append(tags, tag)
			if prefix, found := strings.CutSuffix(tag, ver); found {
				strm.Fetch = fmt.Sprintf("%d %s %s", TAG, prefix, repo)
				strm.LastChk = time.Now()
				strm.LastUpd = time.Now()
				strm.Ver = ver
				return strm
			}
		}
		if len(tags) > 0 {
			l.Warn("Found tags, but cannot determine ver/prefix",
				zap.Strings("tags", tags),
				zap.String("repo", repo),
				zap.String("ver", ver))
		}
		return nil
	}) {
		return true
	}

	// GHFetchType: QL
	// TODO

	l.Warn("No available methods for determining ver", zap.String("repo", repo))
	return false
}
