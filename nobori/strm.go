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
// `strm.go`: determine stream of a package.
package nobori

import (
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mdobak/go-xerrors"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const STRM_MGR_POOL_MAX = 100

type NewStrmManager struct {
	// Receive new stream results
	newStrmPool chan string
	// Requests to get streams
	JobRequest chan *StrmChkJob
	// List of urls of new streams
	strmUrls  [STRM_MGR_POOL_MAX]map[string]struct{}
	jobs      [STRM_MGR_POOL_MAX][]*StrmChkJob
	done      chan StrmChkDoneMsg
	available chan int
}

type StrmChkDoneMsg struct {
	strm *db.Stream
	idx  int
}

type StrmChkJob struct {
	// urls for the stream. If empty, indicates job finished.
	URLs []string
	// obtain the final resulting stream (saved)
	ch  chan<- *db.Stream
	idx int
	// version provided by package
	Ver string
	// manifest functions, each function takes in a package version as argument,
	// return an optional stream (not saved) if confirm
	// register this function via [StrmChkJob.Manifest]()
	//
	// As to why we need to save these functions, if a new job with the same upstream
	// is requested, we can quickly check via these functions whether the version
	// matches, then quit job earlier.
	ManifestFuncs []func(string) *db.Stream
}

var newStrmMgr = &NewStrmManager{}

func (mgr *NewStrmManager) Init() {
	mgr.newStrmPool = make(chan string, STRM_MGR_POOL_MAX)
	mgr.available = make(chan int, STRM_MGR_POOL_MAX)
	mgr.JobRequest = make(chan *StrmChkJob, STRM_MGR_POOL_MAX)
	for i := range STRM_MGR_POOL_MAX {
		mgr.available <- i
		mgr.strmUrls[i] = make(map[string]struct{})
	}
	mgr.done = make(chan StrmChkDoneMsg, 100)
	go mgr.Loop()
}

func (mgr *NewStrmManager) Check(urls []string) *db.Stream {
	ch := make(chan *db.Stream)
	job := &StrmChkJob{
		URLs: util.SliceMap(urls, mangleUrl),
		ch:   ch,
	}
	mgr.JobRequest <- job

	return <-ch
}

func (mgr *NewStrmManager) Loop() {
next_job:
	for len(mgr.done) > 0 || len(mgr.available) == 0 {
		msg := <-mgr.done
		mgr.available <- msg.idx
		defer clear(mgr.jobs[msg.idx][0].URLs) // job finished
		defer clear(mgr.strmUrls[msg.idx])
		defer clear(mgr.jobs[msg.idx])
		if msg.strm == nil {
			for _, job := range mgr.jobs[msg.idx] {
				job.ch <- nil
			}
			goto next_job
		}
		mirrors := util.SliceMap(mgr.jobs[msg.idx][0].URLs, func(url string) db.StreamMirror {
			return db.StreamMirror{
				StreamID: msg.strm.ID,
				Mirror:   url,
			}
		})
		util.Yeet(l, "fail to save stream mirrors", db.DB.Save(mirrors).Error)

		for _, job := range mgr.jobs[msg.idx] {
			job.ch <- msg.strm
		}
	}
	select {
	case job := <-mgr.JobRequest:
		for i, urlMap := range mgr.strmUrls {
			for _, url := range job.URLs {
				if _, ok := urlMap[url]; !ok {
					continue
				}
				for _, u := range job.URLs {
					urlMap[u] = struct{}{}
				}
				job.idx = i
				mgr.jobs[i] = append(mgr.jobs[i], job)
				var strm *db.Stream
				if slices.ContainsFunc(job.ManifestFuncs, func(fn func(url string) *db.Stream) bool {
					return slices.ContainsFunc(job.URLs, func(url string) bool { strm = fn(url); return strm != nil })
				}) {
					// WARN: there's a slight chance `Manifest()` is also sending a `done <-`
					// we should handle this later.
					mgr.done <- StrmChkDoneMsg{
						strm: strm,
						idx:  i,
					}
					db.DB.Save(strm)
				}
				goto next_job
			}
		}
		// TODO: also check database
		job.idx = <-mgr.available
		mgr.jobs[job.idx] = []*StrmChkJob{job}
		l.Debug("run new stream job", zap.Int("job_idx", job.idx))
		go mgr.hdlNewStrm(job)
	default:
		time.Sleep(10 * time.Millisecond)
	}
	goto next_job
}

func (mgr NewStrmManager) hdlNewStrm(job *StrmChkJob) {
	l.Debug("new strm", zap.Strings("urls", job.URLs))
	for _, hdlr := range newStrmHdlrs {
		for _, url := range job.URLs {
			if hdlr(job, url, job.URLs) {
				l.Debug("new strm success", zap.String("url", url))
				return
			}
			if len(job.URLs) == 0 {
				// indicates job finished
				return
			}
		}
	}
	l.Debug("didn't find any existing strm, and no strm supported", zap.Strings("urls", job.URLs))
	mgr.done <- StrmChkDoneMsg{
		strm: nil,
		idx:  job.idx,
	}
}

// Register a manifest function
//
// If this function returns true, job is done, so stop immediately.
func (job *StrmChkJob) Manifest(fn func(string) *db.Stream) bool {
	if len(job.URLs) == 0 {
		// indicates job finished
		return true
	}
	job.ManifestFuncs = append(job.ManifestFuncs, fn)
	if strm := fn(job.Ver); strm != nil {
		db.DB.Save(strm)
		newStrmMgr.done <- StrmChkDoneMsg{
			strm: strm,
			idx:  job.idx,
		}
		l.Debug("job done", zap.Int("job_idx", job.idx))
		return true
	}
	return false
}

var newStrmHdlrs = []func(job *StrmChkJob, url string, urls []string) bool{
	func(job *StrmChkJob, url string, urls []string) bool {
		if !strings.HasPrefix(url, "github.com/") {
			return false
		}
		l.Debug("strm hdl github")
		return GhStrmHdlr(job, url)
	},
}

var mangleUrlRegex = regexp.MustCompile(`^(https?://)(.+)$`)

func mangleUrl(url string) (new string) {
	match := mangleUrlRegex.FindSubmatch([]byte(url))
	if match == nil {
		return url
	}
	return string(match[2])
}

// From https://pkg.go.dev/database/sql#DB,
// "Once DB.Begin is called, the returned Tx is bound to a single connection."
var regpkg_db_mu sync.Mutex

// Register package in db
//
// dbx is used to save only packages.
// WARN: currently we assume the upstream does not change for any packages
func RegPkg(dbx *gorm.DB, p *db.Pkg) error {
	l.Debug("RegPkg", zap.String("pkgname", p.Name))
	if p.StreamID != nil {
		return dbx.Save(p).Error
	}
	url_ch := make(chan string)
	go UpTrace(*p, url_ch)
	urls := map[string]struct{}{}
	for url := range url_ch {
		url = mangleUrl(url)
		urls[url] = struct{}{}
	}
	if len(urls) == 0 {
		return dbx.Save(p).Error
	}

	var mirrors []db.StreamMirror
	urls_slice := slices.Collect(maps.Keys(urls))
	regpkg_db_mu.Lock()
	// PERF: consider join instead of subquery?
	err := dbx.Find(&mirrors, "stream_id IN (SELECT stream_id FROM stream_mirrors WHERE mirror IN ?)", urls_slice).Error
	regpkg_db_mu.Unlock()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			goto new
		} else {
			regpkg_db_mu.Lock()
			dbx.Save(p)
			regpkg_db_mu.Unlock()
			return xerrors.Newf("can't find StreamMirror in db: %w", err)
		}
	}
	if len(mirrors) != 0 {
		return handleExistingStrm(urls, mirrors, p, dbx)
	}
	regpkg_db_mu.Lock()
	defer regpkg_db_mu.Unlock()
	if err := dbx.Save(p).Error; err != nil {
		return err
	}
new:
	// WARN: potential race conditions
	// we can do this after the db query, we suspect the chances of race conditions is near impossible
	// given the stream is first saved, and only then is it removed from the array in the mgr.
	// this hypothesis clearly depends on the speed of dbx.Find() and the num of urls, so we will need
	// to revise this for prod at a later stage.
	// NOTE: for very good reasons, we should run RegPkg for pkgs without StreamID periodically
	newStrmMgr.Check(slices.Collect(maps.Keys(urls)))
	return nil
}

func handleExistingStrm(urls map[string]struct{}, mirrors []db.StreamMirror, p *db.Pkg, dbx *gorm.DB) error {
	var new_mirrors []db.StreamMirror
	l.Debug("handle existing", zap.String("pkgname", p.Name))
	util.SliceEach(mirrors, func(m db.StreamMirror) { urls[m.Mirror] = struct{}{} })
	for _, mirror := range mirrors {
		if mirror.StreamID != mirrors[0].StreamID {
			regpkg_db_mu.Lock()
			defer regpkg_db_mu.Unlock()
			dbx.Model(&db.Pkg{}).Where("stream_id = ?", mirror.StreamID).Update("stream_id", mirrors[0].StreamID)
			mirror.StreamID = mirrors[0].StreamID
			new_mirrors = append(new_mirrors, mirror)
		}
	}
	for url := range urls {
		if !slices.ContainsFunc(mirrors, func(mirror db.StreamMirror) bool { return mirror.Mirror == url }) {
			new_mirrors = append(new_mirrors, db.StreamMirror{
				StreamID: mirrors[0].StreamID,
				Mirror:   url,
			})
			urls[url] = struct{}{}
		}
	}
	p.StreamID = &mirrors[0].StreamID
	regpkg_db_mu.Lock()
	defer regpkg_db_mu.Unlock()
	if err := dbx.Save(p).Error; err != nil {
		return xerrors.Newf("can't save package in db (pkgid %s): %w", p.ID.String(), err)
	}
	if len(new_mirrors) != 0 {
		if err := db.DB.Save(new_mirrors).Error; err != nil {
			return xerrors.Newf("can't save mirrors in db (strmid %s): %w", mirrors[0].StreamID.String(), err)
		}
	}
	l.Debug("reg success", zap.String("pkgid", p.ID.String()))
	return nil
}
