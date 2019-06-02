package twilter

import (
	"context"
	"github.com/dghubble/go-twitter/twitter"
	"time"
)

const (
	defaultSize      = 200
	defaultIteration = 16
	defaultLimit     = time.Hour
)

// Loader loads all tweets and filters tweets.
type Loader struct {
	targetId     string
	size         int
	maxIteration int
	limitInital  time.Duration
}

// LoaderOption ...
type LoaderOption struct {
	Size         int
	MaxIteration int
	LimitInital  time.Duration
}

// NewLoader returns Loader
func NewLoader(targetId string, option *LoaderOption) *Loader {
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
	if option.LimitInital == 0 {
		option.LimitInital = defaultLimit
	}

	return &Loader{
		targetId:     targetId,
		size:         option.Size,
		maxIteration: option.MaxIteration,
		limitInital:  option.LimitInital,
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
		var timeline []twitter.Tweet
		// Get tweets of the user
		// https://developer.twitter.com/en/docs/tweets/timelines/api-reference/get-statuses-user_timeline.html
		timeline, _, err = client.Timelines.UserTimeline(&twitter.UserTimelineParams{
			ScreenName:      l.targetId,
			TrimUser:        &trueValue,
			IncludeRetweets: &trueValue,
			ExcludeReplies:  &falseValue,
			MaxID:           maxId,
			SinceID:         sinceId,
			Count:           l.size,
		})

		if err != nil {
			// TODO: check rate limit error and continue
			return nil, nil, err
		}

		// set latest tweet
		if latest == nil && len(timeline) > 0 {
			latest = &timeline[0]
		}

		for i := range timeline {
			if sinceId == 0 {
				// check whether tweet is older than limit.
				if createdAt, err := timeline[i].CreatedAtTime(); err == nil && time.Now().Add(-l.limitInital).After(createdAt) {
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
