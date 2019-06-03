package twilter

import (
	"context"
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"net/http"
	"time"
)

const (
	defaultSize      = 200
	defaultIteration = 16
	defaultFallback  = time.Hour
)

// Loader loads all tweets and filters tweets.
type Loader struct {
	screenName   string
	userId       int64
	size         int
	maxIteration int
	fallback     time.Duration
}

// LoaderOption ...
type LoaderOption struct {
	Size         int
	MaxIteration int
	Fallback     time.Duration
}

// NewLoaderScreenName returns Loader for screenName (string)
func NewLoaderScreenName(screenName string, option *LoaderOption) *Loader {
	return newLoader(screenName, 0, option)
}

// NewLoader returns Loader for userId (int64)
func NewLoader(userId int64, option *LoaderOption) *Loader {
	return newLoader("", userId, option)
}

func newLoader(screenName string, userId int64, option *LoaderOption) *Loader {
	// set default options
	if option == nil {
		option = new(LoaderOption)
	}
	if option.Size == 0 {
		option.Size = defaultSize
	}
	if option.MaxIteration == 0 {
		option.MaxIteration = defaultIteration
	}
	if option.Fallback == 0 {
		option.Fallback = defaultFallback
	}

	return &Loader{
		screenName:   screenName,
		userId:       userId,
		size:         option.Size,
		maxIteration: option.MaxIteration,
		fallback:     option.Fallback,
	}
}

// Load loads tweets since sinceId from UserTimeline API and filters tweets.
// params : `sinceId` : load tweets since sinceId. ignored when sinceId is 0.
// params : `filters` : slice of filter.Filter
// return : `tweets`  : filtered tweets by Loader.filters which order is new to old
// return : `latest`  : lastest tweet. nil when no new tweet found.
// return : `err`     : error from UserTimeline API
func (l *Loader) Load(ctx context.Context, client *twitter.Client, sinceId int64, filters []Filter) (tweets []twitter.Tweet, latest *twitter.Tweet, err error) {
	var (
		trueValue        = true
		falseValue       = false
		maxId      int64 = 0
	)

	// 3200 tweets is available on User Timeline API at the most
totalLoop:
	for r := 0; r < l.maxIteration; r++ {
		var (
			timeline []twitter.Tweet
			resp     *http.Response
		)
		// Get tweets of the user
		// https://developer.twitter.com/en/docs/tweets/timelines/api-reference/get-statuses-user_timeline.html
		timeline, resp, err = client.Timelines.UserTimeline(&twitter.UserTimelineParams{
			ScreenName:      l.screenName,
			UserID:          l.userId,
			TrimUser:        &trueValue,
			IncludeRetweets: &trueValue,
			ExcludeReplies:  &falseValue,
			MaxID:           maxId,
			SinceID:         sinceId,
			Count:           l.size,
		})

		// TODO: check rate limit error and continue
		if err != nil {
			return nil, nil, err
		} else if resp.StatusCode >= 300 {
			// twitter.APIError is not reliable when error response body format from twitter is not valid.
			return nil, nil, fmt.Errorf("request to user timeline : %v", resp.Status)
		}

		// set latest tweet
		if latest == nil && len(timeline) > 0 {
			latest = &timeline[0]
		}

		for i := range timeline {
			if sinceId == 0 {
				// check whether tweet is older than fallback.
				if createdAt, err := timeline[i].CreatedAtTime(); err == nil && time.Now().Add(-l.fallback).After(createdAt) {
					// finish traversing.
					break totalLoop
				}
			}

			// check filters matches or not.
			for _, f := range filters {
				if f.NotMatch(&timeline[i]) {
					// break filters loop and check next tweet
					break
				} else if f.Match(&timeline[i]) {
					// copy tweet to result array
					// not set pointer because timeline will not be GCed when next timeline search.
					tweets = append(tweets, timeline[i])

					// break filters loop and check next tweet
					break
				}
			}
		}

		if len(timeline) == 0 {
			// no more tweets older. finish traversing.
			break
		}

		lastTweet := &timeline[len(timeline)-1]

		// next max_id is 1 smaller than id of oldest tweet in the range.
		maxId = lastTweet.ID - 1
	}

	return tweets, latest, nil
}
