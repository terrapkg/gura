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
// Periodically synchronizes data from upstream sources and schedules future fetches
// using a specialised backoff strategy ([calcTimeout]).
//
// The name nobori (上り) is Japanese for "up".
//
// This file implements the main scheduling, timeout calculation, and fetch loop logic for upstream requests.

package nobori

import (
	"math"
	"time"

	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/util"
)

var l = util.SetupLog("nobori")
var scheduler = util.NewScheduler()

// Central channel for scheduling upstream stream fetches
var queue chan db.Stream = make(chan db.Stream)

// Upstream handlers
// Each handler function should return a channel.
// Sending a message to the channel indicates the swimmer is ready.
var swimmers = []func() chan struct{}{GhSwimInit}

// Calculate the timeout duration
//
// A power-law function is used to calculate the timeout duration.
// It starts with 1 minute minimum and increases logarithmically,
// until it reaches a maximum of 1 hour.
func calcTimeout(lastChk time.Time) time.Duration {
	// 0 min ⇒ 1 min
	// 1 day ⇒ 10 min
	// 1 year ⇒ 1 hour
	// From deepseek, we can use the power-law function: f(t) = a·t^b + c
	// a = 9 / 1440ᵇ
	// b = ln(59/9)/ln(365)
	// c = 1
	t := time.Since(lastChk).Minutes()
	f_t := 1 + 9*math.Pow(t/1440, math.Log(59/9)/math.Log(365))
	return time.Duration(f_t * float64(time.Minute))
}

// Wait until the next scheduled time for a stream and then enqueues it for processing
func schedule(stream db.Stream) {
	t := stream.LastChk.Add(calcTimeout(stream.LastChk))
	scheduler.Schedule(t, func() {
		queue <- stream
	})
}

// Start the fetch loop for upstream metadata fetching
//
// Return when nobori is ready.
func StartFetchLoop() {
	l.Info("preparing fetch loop")
	ready_chs := util.SliceMap(swimmers, func(swimmer func() chan struct{}) chan struct{} { return swimmer() })
	var strms []db.Stream
	r := db.DB.Find(&strms)
	util.Yeet(l, "can't find streams", r.Error)

	util.SliceEach(ready_chs, func(ch chan struct{}) { <-ch })
	newStrmMgr.Init()

	go fetchLoop(strms)
	l.Info("nobori ready")
}

// Main loop for upstream metadata fetching
//
// Streams are processed from the queue as their scheduled time arrives.
func fetchLoop(strms []db.Stream) {
	for _, stream := range strms {
		schedule(stream)
	}
	githubJobs := []NoboriGitHubJob{}

	for {
		githubJobs = handleCompletedGitHubJobs(githubJobs)
		var stream db.Stream
		select {
		case stream = <-queue:
		default:
			time.Sleep(10 * time.Millisecond)
			continue
		}
		switch stream.Forge {
		case db.GitHub:
			job := GHJob{Result: make(chan string), Init: false, Fetch: stream.Fetch}
			ghPool <- job
			githubJobs = append(githubJobs, NoboriGitHubJob{
				stream: stream,
				job:    job,
			})
		}
	}
}

func handleCompletedGitHubJobs(jobs []NoboriGitHubJob) (remainingJobs []NoboriGitHubJob) {
	for _, gh := range jobs {
		select {
		case ver, ok := <-gh.job.Result:
			schedule(gh.stream)
			if !ok {
				l.Error("cannot fetch stream")
				continue // Don't re-add this job
			}
			if ver != gh.stream.Ver {
				gh.stream.Ver = ver
				gh.stream.LastUpd = time.Now()
			}
			gh.stream.LastChk = time.Now()
			util.Yeet(ghl, "cannot save stream", db.DB.Save(gh.stream).Error)
		default:
			remainingJobs = append(remainingJobs, gh)
		}
	}
	return
}

type NoboriGitHubJob struct {
	stream db.Stream
	job    GHJob
}
