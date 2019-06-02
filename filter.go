package twilter

import (
	"github.com/dghubble/go-twitter/twitter"
)

// Filter filters tweets.
type Filter interface {
	Match(tweet *twitter.Tweet) bool
	NotMatch(tweet *twitter.Tweet) bool
}

// AllFilter pass all tweets.
type AllFilter struct{}

// Match always returns true
func (_ AllFilter) Match(tweet *twitter.Tweet) bool {
	return true
}

// NotMatch always returns false
func (_ AllFilter) NotMatch(tweet *twitter.Tweet) bool {
	return false
}

// PhotoFilter filters photo tweets
type PhotoFilter struct{}

// Match ...
func (_ PhotoFilter) Match(tweet *twitter.Tweet) bool {
	if tweet.ExtendedEntities != nil {
		for i := range tweet.ExtendedEntities.Media {
			if tweet.ExtendedEntities.Media[i].Type == "photo" {
				return true
			}
		}
	} else if tweet.Entities != nil && len(tweet.Entities.Media) > 0 {
		// fallback to entities if extended_entities not exists.
		for i := range tweet.Entities.Media {
			if tweet.Entities.Media[i].Type == "photo" {
				return true
			}
		}
	}
	return false
}

// NotMatch always returns false
func (_ PhotoFilter) NotMatch(tweet *twitter.Tweet) bool {
	return false
}

// VideoFilter filters video tweets
type VideoFilter struct{}

// Match ...
func (_ VideoFilter) Match(tweet *twitter.Tweet) bool {
	if tweet.ExtendedEntities != nil {
		for i := range tweet.ExtendedEntities.Media {
			if tweet.ExtendedEntities.Media[i].Type == "video" {
				return true
			}
		}
	} else if tweet.Entities != nil && len(tweet.Entities.Media) > 0 {
		// fallback to entities if extended_entities not exists.
		for i := range tweet.Entities.Media {
			if tweet.Entities.Media[i].Type == "video" {
				return true
			}
		}
	}
	return false
}

// NotMatch always returns false
func (_ VideoFilter) NotMatch(tweet *twitter.Tweet) bool {
	return false
}
