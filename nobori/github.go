package nobori

import (
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/terrapkg/gura/db"
)

// ? https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#about-secondary-rate-limits
// no more than 100 parallel requests
var pool = make(chan db.Stream, 100)
var tokens []Token

type Token struct {
	key   string
	quota int16
	reset time.Time
}

func fetchGitHub(stream db.Stream) {
	pool <- stream
}

func fillGitHubTokens() Token {
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
	return tokens[0]
}

func noMoreFish(token Token) bool {
	return token.quota == 0
}
func thanksForAllTheFish(token *Token, token_idx *int) {
	*token_idx = (*token_idx + 1) % len(tokens)
	*token = tokens[*token_idx]
}
func waitForFish(token Token) {
	if token.quota == 0 {
		time.Sleep(time.Until(token.reset))
	}
}

func updToken(token *Token, resp *http.Response) {
	quota, err := strconv.ParseInt(resp.Header.Get("x-ratelimit-remaining"), 10, 16)
	if err != nil {
		log.Fatalf("github: strconv x-ratelimit-remaining: %v", err, *token)
		return
	}
	reset, err := strconv.ParseInt(resp.Header.Get("x-ratelimit-reset"), 10, 64)
	if err != nil {
		log.Fatalf("github: strconv x-ratelimit-reset: %v", err, *token)
		return
	}
	token.quota = int16(quota)
	token.reset = time.Unix(reset, 0)
}

// the shark shall initialise the funny
func swimGitHub() {
	token_idx := 0
	for token := fillGitHubTokens(); noMoreFish(token); thanksForAllTheFish(&token, &token_idx) {
		waitForFish(token)
		for {
			stream := <-pool
			req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+stream.Fetch+"/tags", nil)
			if err != nil {
				log.Printf("github: req [%s]: %v", stream.Fetch, err)
				continue
			}
			req.Header.Add("Authorization", "Bearer "+token.key)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("github: resp [%s]: %v", stream.Fetch, err)
				continue
			}
			updToken(&token, resp)
			// TODO: update stream
		}
	}
}
