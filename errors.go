package twilter

import (
	"github.com/dghubble/go-twitter/twitter"
	"log"
	"net/http"
	"strconv"
	"time"
)

// https://developer.twitter.com/en/docs/basics/response-codes.html
const (
	codeAlreadyRetweeted    = 327
	codeRetweetNotPermitted = 328
)

// IsAlreadyRetweeted checks the error is "already retweeted" error
func IsAlreadyRetweeted(err error) bool {
	if apierr, ok := err.(*twitter.APIError); ok {
		for _, e := range apierr.Errors {
			if e.Code == codeAlreadyRetweeted {
				return true
			}
		}
	}
	return false
}

// IsRetweetNotPermitted checks the error is "not permitted" error
func IsRetweetNotPermitted(err error) bool {
	if apierr, ok := err.(*twitter.APIError); ok {
		for _, e := range apierr.Errors {
			if e.Code == codeRetweetNotPermitted {
				return true
			}
		}
	}
	return false
}

func IsRateLimit(resp *http.Response) (limited bool, sleep time.Duration) {
	// Twitter API returns 403, 420, 429 http status code when rate limited.
	// https://developer.twitter.com/en/docs/basics/response-codes.html
	code := resp.StatusCode
	if code == 403 || code == 420 || code == 429 {
		remain := resp.Header.Get("X-Rate-Limit-Remaining")
		resetStr := resp.Header.Get("X-Rate-Limit-Reset")
		if remain == "0" && resetStr != "" {
			// this request is rate limited.
			resetInt, err := strconv.ParseInt(resetStr, 10, 64)
			if err != nil {
				log.Println("failed to parse X-Rate-Limit-Reset : ", err)
				return false, 0
			}
			resetTime := time.Unix(resetInt, 0)
			return true, resetTime.Sub(time.Now())
		}
	}
	return false, 0
}
