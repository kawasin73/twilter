package main

import (
	"context"
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/go-redis/redis"
	"github.com/kawasin73/htask"
	"github.com/kawasin73/twilter"
	"log"
	"time"
)

type Task struct {
	oauthConfig *oauth1.Config
	oauthToken  *oauth1.Token
	loader      *twilter.Loader
	idStore     *idStore
	filters     []twilter.Filter
	interval    time.Duration
	timeout     time.Duration
}

func setupTask(ctx context.Context, config *oauth1.Config, token *oauth1.Token, redisClient *redis.Client, t *target, interval, timeout, fallback time.Duration) (*Task, error) {
	// initialize task
	task := &Task{
		oauthConfig: config,
		oauthToken:  token,
		filters:     t.filters,
		interval:    interval,
		timeout:     timeout,
	}

	// convert screenName to userId
	falseValue := false
	twitterClient := task.twitterClient(ctx)
	user, resp, err := twitterClient.Users.Show(&twitter.UserShowParams{
		ScreenName:      t.screenName,
		IncludeEntities: &falseValue,
	})
	if err == nil && resp.StatusCode >= 300 {
		err = fmt.Errorf("request to show user : %v", resp.Status)
	}
	if err != nil {
		return nil, fmt.Errorf("convert screenName to userId : %v", err)
	}
	targetId := user.ID

	// create loader
	task.loader = twilter.NewLoader(targetId, &twilter.LoaderOption{Fallback: fallback})

	// load latestId from Redis
	is, err := createIdStore(redisClient, targetId)
	if err != nil {
		return nil, fmt.Errorf("create id store : %v", err)
	}
	task.idStore = is

	return task, nil
}

func (t *Task) Start(ctx context.Context, sche *htask.Scheduler) error {
	return sche.Set(ctx.Done(), time.Now(), t.buildTask(ctx, sche))
}

func (t *Task) buildTask(ctx context.Context, sche *htask.Scheduler) func(_ time.Time) {
	return func(_ time.Time) {
		t.Exec(ctx)
		if err := sche.Set(ctx.Done(), time.Now().Add(t.interval), t.buildTask(ctx, sche)); err != nil {
			log.Println("failed to set scheduler :", err)
		}
	}
}

// twitterClient create new twitter.Client
func (t *Task) twitterClient(ctx context.Context) *twitter.Client {
	httpClient := t.oauthConfig.Client(ctx, t.oauthToken)
	return twitter.NewClient(httpClient)
}

// Exec executes task. load and filter tweets and retweet filtered tweets.
func (t *Task) Exec(ctx context.Context) {
	log.Println("start loading...")
	// set timeout to context
	tctx, cancel := context.WithDeadline(ctx, time.Now().Add(t.timeout))
	defer cancel()

	// setup twitter client
	client := t.twitterClient(tctx)

	// load tweets
	tweets, latest, err := t.loader.Load(tctx, client, t.idStore.get(), t.filters)
	if err != nil {
		log.Println("failed to load tweets :", err)
		return
	}
	log.Printf("load %d items\n", len(tweets))

	// retweet tweets
	var trueValue = true
	for i := len(tweets) - 1; i >= 0; i-- {
		tw := &tweets[i]

		if tw.Retweeted {
			// if already retweeted then unretweet and retweet again.
			// TODO: handle rate limit error
			log.Printf("tweet (%d) is already retweeted. so unretweet.\n", tw.ID)
			_, resp, err := client.Statuses.Unretweet(tw.ID, &twitter.StatusUnretweetParams{
				TrimUser: &trueValue,
			})
			if err == nil && resp.StatusCode >= 300 {
				// twitter.APIError is not reliable when error response body format from twitter is not valid.
				err = fmt.Errorf("request to unretweet : %v", resp.Status)
			}
			// even if the tweet is not retweeted, no error occurs.
			if err != nil {
				log.Println("failed to unretweet :", err)
				return
			}
		}

		// retweet
		_, resp, err := client.Statuses.Retweet(tw.ID, &twitter.StatusRetweetParams{
			TrimUser: &trueValue,
		})
		// TODO: handle rate limit error
		if err == nil && resp.StatusCode >= 300 {
			// twitter.APIError is not reliable when error response body format from twitter is not valid.
			err = fmt.Errorf("request to retweet : %v", resp.Status)
		}

		if err != nil {
			if twilter.IsAlreadyRetweeted(err) || twilter.IsRetweetNotPermitted(err) {
				// retweet failed. but skip this tweet.
				log.Println("retweet failed :", err)
				log.Println("skip to retweet :", tw.ID)
			} else {
				log.Println("failed to retweet :", err)
				return
			}
		} else {
			log.Println("retweeted :", tw.ID)
			log.Println("text      :", tw.Text)
		}

		// update latestId
		if err = t.idStore.update(tw.ID); err != nil {
			log.Println("failed to save latest id :", err)
		}
	}

	// update latestId
	if latest != nil {
		if err = t.idStore.update(latest.ID); err != nil {
			log.Println("failed to save latest id :", err)
		}
	}
}

