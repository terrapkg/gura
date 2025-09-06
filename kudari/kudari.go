// Handle downstream requests.
//
// This fetches and handles repo metadata routinely.
// The name kudari (下り) is Japanese for "down".

package kudari

import (
	"context"
	"log"
	"time"

	"github.com/terrapkg/gura/db"
	"gorm.io/gorm"
)

const fetchRepoTimer = 5000_000 * 60 // 5min

func fetch(repo db.Repo) {
	log.Println("kudari: fetching:", repo.ID)
	switch repo.Type {
	case db.Rpm:
		rpmFetch(repo)
	}
	n, err := gorm.G[db.Repo](db.DB).Where("id = ?", repo.ID).Update(context.Background(), "upd_at", time.Now())
	if err != nil {
		log.Println("kudari: err while mut upd_at:", err)
	}
	if n != 1 {
		log.Printf("kudari: bug: mut upd_at: n=%d (Repo.ID = %s)\n", n, repo.ID)
	}
	log.Println("kudari: done:", repo.ID)
	time.Sleep(fetchRepoTimer)
	go fetch(repo)
}

func rpmFetch(repo db.Repo) {
}

func FetchLoop() {
	log.Println("kudari: scheduling fetch loop")
	var repos []db.Repo
	r := db.DB.Find(&repos)
	if r.Error != nil {
		log.Fatalln("kudari: err:", r.Error)
	}
	// schedule fetch
	for i, repo := range repos {
		timeSinceFetch := time.Since(repo.UpdAt).Nanoseconds()
		if fetchRepoTimer < timeSinceFetch {
			log.Printf("kudari: [%d/%d] run now (%s UpdAt %s)\n", i, r.RowsAffected, repo.ID, repo.UpdAt)
			go fetch(repo)
		}
		go func() {
			dur := time.Duration(fetchRepoTimer - timeSinceFetch)
			log.Printf("kudari: [%d/%d] sched dur=%.0fs (Repo.ID = %s)\n", i, r.RowsAffected, dur.Seconds(), repo.ID)
			time.Sleep(dur)
			go fetch(repo)
		}()
	}
}
