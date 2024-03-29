package twilter

import (
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"strings"
)

// Filter filters tweets.
type Filter interface {
	Match(tweet *twitter.Tweet) bool
	String() string
}

// AllFilter pass all tweets.
type AllFilter struct{}

// String returns all
func (_ AllFilter) String() string {
	return "all"
}

// Match always returns true
func (_ AllFilter) Match(tweet *twitter.Tweet) bool {
	return true
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

// String returns photo
func (_ PhotoFilter) String() string {
	return "photo"
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

// String returns video
func (_ VideoFilter) String() string {
	return "video"
}

// RTFilter filters retweeted tweets
type RTFilter struct{}

// Match ...
func (_ RTFilter) Match(tweet *twitter.Tweet) bool {
	return tweet.RetweetedStatus != nil
}

// String returns rt
func (_ RTFilter) String() string {
	return "rt"
}

// QTFilter filters quoted tweets
type QTFilter struct {}

// Match ...
func (_ QTFilter) Match(tweet *twitter.Tweet) bool {
	return tweet.QuotedStatusID > 0
}

// String returns qt
func (_ QTFilter) String() string {
	return "qt"
}

// NotFilter return toggled result of Origin
type NotFilter struct{
	Original Filter
}

// Match ...
func (f NotFilter) Match(tweet *twitter.Tweet) bool {
	return !f.Original.Match(tweet)
}

// String returns not
func (f NotFilter) String() string {
	return fmt.Sprintf("not(%v)", f.Original)
}

// AndFilter return AND result of all filters
type AndFilter struct{
	Filters []Filter
}

// Match ...
func (f AndFilter) Match(tweet *twitter.Tweet) bool {
	if len(f.Filters) == 0 {
		return false
	}
	for _, ff := range f.Filters {
		if !ff.Match(tweet) {
			return false
		}
	}
	return true
}

// String returns and
func (f AndFilter) String() string {
	ss := make([]string, len(f.Filters))
	for i, ff := range f.Filters {
		ss[i] = ff.String()
	}
	return fmt.Sprintf("and(%v)", strings.Join(ss, ","))
}

// OrFilter return OR result of all filters
type OrFilter struct{
	Filters []Filter
}

// Match ...
func (f OrFilter) Match(tweet *twitter.Tweet) bool {
	if len(f.Filters) == 0 {
		return false
	}
	for _, ff := range f.Filters {
		if ff.Match(tweet) {
			return true
		}
	}
	return false
}

// String returns or
func (f OrFilter) String() string {
	ss := make([]string, len(f.Filters))
	for i, ff := range f.Filters {
		ss[i] = ff.String()
	}
	return fmt.Sprintf("or(%v)", strings.Join(ss, ","))
}
