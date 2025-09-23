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
	"go.uber.org/zap"
)

var l = util.SetupLog("nobori")

// Central channel for scheduling upstream stream fetches
var queue chan db.Stream = make(chan db.Stream)

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
	l.Debug("sched", zap.String("stream", stream.ID.String()), zap.Time("until", t))
	time.Sleep(time.Until(t))
	queue <- stream
}

// Main loop for upstream metadata fetching
//
// Streams are processed from the queue as their scheduled time arrives.
func FetchLoop() {
	go GhSwim()
	var streams []db.Stream
	r := db.DB.Find(&streams)
	util.Yeet(l, "can't find streams", r.Error)
	for _, stream := range streams {
		go schedule(stream)
	}
	for {
		switch stream := <-queue; stream.Forge {
		case db.GitHub:
			go GhFetch(stream)
		}
	}
}
