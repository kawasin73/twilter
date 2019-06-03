package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/kawasin73/htask"
	"github.com/kawasin73/twilter"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

func unwrapArgs(args string) (string, error) {
	// remove ( and )
	if len(args) < 3 || args[0] != '(' || args[len(args)-1] != ')' {
		return "", fmt.Errorf("args not have ()")
	}
	return args[1:len(args)-1], nil
}

func parseFilters(value string, sep string) ([]twilter.Filter, error) {
	// parse multi filters separated by sep
	values := strings.Split(value, sep)
	var filters []twilter.Filter
	for _, v := range values {
		filter, err := parseFilter(v)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	return filters, nil
}

func parseFilter(value string) (twilter.Filter, error) {
	// parse each filter
	switch {
	case value == "photo":
		return twilter.PhotoFilter{}, nil
	case value == "video":
		return twilter.VideoFilter{}, nil
	case value == "rt":
		return twilter.RTFilter{}, nil
	case strings.HasPrefix(value, "not"):
		args, err := unwrapArgs(value[3:])
		if err != nil {
			return nil, err
		}
		filter, err := parseFilter(args)
		if err != nil {
			return nil, err
		}
		return twilter.NotFilter{filter}, nil
	case strings.HasPrefix(value, "and"):
		if args, err := unwrapArgs(value[3:]); err != nil {
			return nil, err
		} else if filters, err := parseFilters(args, ","); err != nil {
			return nil, err
		} else {
			return twilter.AndFilter{filters}, nil
		}
	case strings.HasPrefix(value, "or"):
		if args, err := unwrapArgs(value[2:]); err != nil {
			return nil, err
		} else if filters, err := parseFilters(args, ","); err != nil {
			return nil, err
		} else {
			return twilter.AndFilter{filters}, nil
		}
	default:
		// filter is invalid
		return nil, fmt.Errorf("filter \"%v\" is invalid", value)
	}
}

type target struct {
	screenName string
	filters    []twilter.Filter
}

type targetValue map[string]*target

func (tv targetValue) String() string {
	return "target values"
}

func (tv targetValue) Set(value string) error {
	// get screen_name
	idx := strings.Index(value, ":")
	if idx < 0 {
		return fmt.Errorf("target has no screenName nor filter")
	}
	screenName := value[:idx]

	// get filters
	filters, err := parseFilters(value[idx+1:], "/")
	if err != nil {
		return err
	}

	// get target
	t, ok := tv[screenName]
	if !ok {
		// create new target
		t = &target{
			screenName: screenName,
		}
		tv[screenName] = t
	}

	// set filters
	t.filters = append(t.filters, filters...)

	return nil
}

func main() {
	var (
		consumerKey    = os.Getenv("TWITTER_CONSUMER_KEY")
		consumerSecret = os.Getenv("TWITTER_CONSUMER_SECRET")
		accessToken    = os.Getenv("TWITTER_ACCESS_TOKEN")
		accessSecret   = os.Getenv("TWITTER_ACCESS_TOKEN_SECRET")
	)

	// setup command line option flags
	flagTargets := make(targetValue)
	flagInterval := flag.Int("interval", 10, "interval between monitoring (minutes)")
	flagFallback := flag.Int("fallback", 10, "start filtering tweets fallback minutes ago if no checkpoint (minutes)")
	flagTimeout := flag.Int("timeout", 5, "timeout for each monitoring + retweet loop (minutes)")
	flag.Var(flagTargets, "target", "list of targets. target format = \"<screen_name>:<filter>[/<filter>]\"  filter format = \"<filter_name>[(<attribute>[,<attribute>])]\"")
	flagRedis := flag.String("redis", "", "redis url (optional)")

	flag.Parse()

	// convert parsed flags
	interval := time.Duration(*flagInterval) * time.Minute
	fallback := time.Duration(*flagFallback) * time.Minute
	timeout := time.Duration(*flagTimeout) * time.Minute
	_ = flagRedis

	// setup wait group
	var wg sync.WaitGroup
	defer wg.Wait()

	// setup context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create twitter auth context
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)

	// setup scheduler
	sche := htask.NewScheduler(&wg, 0)
	defer sche.Close()

	for _, t := range flagTargets {
		// create task
		task := setupTask(t, config, token, interval, timeout, fallback)

		// start task.
		err := task.Start(ctx, sche)
		if err != nil {
			log.Panic(err)
		}
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

func setupTask(t *target, config *oauth1.Config, token *oauth1.Token, interval, timeout, fallback time.Duration) *Task {
	loader := twilter.NewLoaderScreenName(t.screenName, &twilter.LoaderOption{Fallback: fallback})
	task := &Task{
		oauthConfig: config,
		oauthToken:  token,
		loader:      loader,
		latestId:    0,
		filters:     t.filters,
		interval:    interval,
		timeout:     timeout,
	}

	// TODO: set latestId from Redis

	return task
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
