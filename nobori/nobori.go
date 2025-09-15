// Handle upstream requests.
//
// This fetches metadata from different upstream projects routinely.
// The name nobori (上り) is Japanese for "up".

package nobori

import (
	"log"
	"math"
	"time"

	"github.com/terrapkg/gura/db"
)

var queue chan db.Stream = make(chan db.Stream)

// Calculate the timeout duration.
//
// A logarithmic function is used to calculate the timeout duration.
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

func schedule(stream db.Stream) {
	time.Sleep(time.Until(stream.LastChk.Add(calcTimeout(stream.LastChk))))
	queue <- stream
}

func FetchLoop() {
	var streams []db.Stream
	if r := db.DB.Find(&streams); r.Error != nil {
		log.Fatalln("nobori: fatal:", r.Error)
	}
	for _, stream := range streams {
		go schedule(stream)
	}
	for {
		switch stream := <-queue; stream.Forge {
			case db.GitHub:
				go fetchGitHub(stream)
		}
	}
}
