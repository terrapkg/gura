// kudari: Fetch and handle downstream repository metadata
//
// This package contains logic for periodically synchronizing package information
// and updating the local database accordingly.
//
// The name kudari (下り) is Japanese for "down".
package kudari

import (
	"context"
	"time"

	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var l = util.SetupLog("kudari")

// const fetchRepoTimer = 3e11 // 5min
const fetchRepoTimer = 3e10 // 30s

func fetch(repo db.Repo) {
	defer func() {
		time.Sleep(fetchRepoTimer)
		go fetch(repo)
	}()
	l.Info("fetching repository", zap.String("repoID", repo.ID))
	switch repo.Type {
	case db.Rpm:
		rpmFetch(repo)
	}
	n, err := gorm.G[db.Repo](db.DB).Where("id = ?", repo.ID).Update(context.Background(), "upd_at", time.Now())
	if util.Yeet(l, "error while updating upd_at", err, zap.String("repoID", repo.ID), zap.Error(err)) {
		return
	}
	if n != 1 {
		l.DPanic("bug: mut upd_at", zap.Int("n", n), zap.String("repoID", repo.ID))
		return
	}
	l.Info("done fetching repository", zap.String("repoID", repo.ID))
}

func FetchLoop() {
	l.Info("scheduling fetch loop")
	var repos []db.Repo
	r := db.DB.Find(&repos)
	if r.Error != nil {
		l.Fatal("error fetching repositories", zap.Error(r.Error))
	}
	// schedule fetch
	for i, repo := range repos {
		timeSinceFetch := time.Since(repo.UpdAt).Nanoseconds()
		if fetchRepoTimer < timeSinceFetch {
			l.Info("run now", zap.Int("index", i+1), zap.Int64("total", r.RowsAffected), zap.String("repoID", repo.ID), zap.Time("UpdAt", repo.UpdAt))
			go fetch(repo)
			continue
		}
		go func(i int, repo db.Repo) {
			dur := time.Duration(fetchRepoTimer - timeSinceFetch)
			l.Info("scheduled fetch", zap.Int("index", i+1), zap.Int64("total", r.RowsAffected), zap.Float64("duration_seconds", dur.Seconds()), zap.String("repoID", repo.ID))
			time.Sleep(dur)
			go fetch(repo)
		}(i, repo)
	}
}
