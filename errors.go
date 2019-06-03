package twilter

import "github.com/dghubble/go-twitter/twitter"

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
