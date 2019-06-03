package main

import (
	"context"
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/kawasin73/htask"
	"github.com/kawasin73/twilter"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"
)

func main() {
	var (
		consumerKey    = os.Getenv("TWITTER_CONSUMER_KEY")
		consumerSecret = os.Getenv("TWITTER_CONSUMER_SECRET")
		accessToken    = os.Getenv("TWITTER_ACCESS_TOKEN")
		accessSecret   = os.Getenv("TWITTER_ACCESS_TOKEN_SECRET")
		wg             sync.WaitGroup
		duration       = time.Minute
		limitInital    = 48 * time.Hour
	)
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// TODO: create task from config and multi task
	// create Task
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	loader := twilter.NewLoaderScreenName("kawasin73", &twilter.LoaderOption{LimitInital: limitInital})
	filters := []twilter.Filter{twilter.PhotoFilter{}}
	task := Task{
		oauthConfig: config,
		oauthToken:  token,
		loader:      loader,
		latestId:    0,
		filters:     filters,
		interval:    duration,
		timeout:     2 * time.Minute,
	}

	sche := htask.NewScheduler(&wg, 0)
	defer sche.Close()

	// start task.
	err := task.Start(ctx, sche)
	if err != nil {
		log.Panic(err)
	}

	// setup signal.
	signal.Ignore()
	chsig := make(chan os.Signal, 1)
	// watch SIGINT
	signal.Notify(chsig, os.Interrupt)

	// wait until signal come.
	select {
	case <-chsig:
		// signal (SIGINT) has come.
		log.Println("shutdown...")
	}
}

type Task struct {
	oauthConfig *oauth1.Config
	oauthToken  *oauth1.Token
	loader      *twilter.Loader
	latestId    int64
	filters     []twilter.Filter
	interval    time.Duration
	timeout     time.Duration
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

func (t *Task) Exec(ctx context.Context) {
	log.Println("start loading...")
	// set timeout to context
	tctx, cancel := context.WithDeadline(ctx, time.Now().Add(t.timeout))
	defer cancel()

	// setup twitter client
	httpClient := t.oauthConfig.Client(tctx, t.oauthToken)
	client := twitter.NewClient(httpClient)

	// load tweets
	tweets, latest, err := t.loader.Load(tctx, client, t.latestId, t.filters)
	if err != nil {
		log.Println("failed to load tweets :", err)
		return
	}
	log.Printf("load %d items\n", len(tweets))

	var trueValue = true
	// retweet tweets
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
		t.latestId = tw.ID
		// TODO: save to persistent layer
	}

	// update latestId
	if latest != nil {
		t.latestId = latest.ID
		// TODO: save to persistent layer
	}
}
