package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/go-redis/redis"
	"github.com/kawasin73/htask"
	"github.com/kawasin73/twilter"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"
)

func unwrapArgs(args string) (string, error) {
	// remove ( and )
	if len(args) < 3 || args[0] != '(' || args[len(args)-1] != ')' {
		return "", fmt.Errorf("args not have ()")
	}
	return args[1 : len(args)-1], nil
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
	str := ""
	for name, t := range tv {
		str += fmt.Sprintf("%s:%v,", name, t.filters)
	}
	return str
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

func setupRedis(redisUrl string) (*redis.Client, error) {
	// if empty then no redis mode
	if redisUrl == "" {
		return nil, nil
	}

	// parse redis url
	u, err := url.Parse(redisUrl)
	if err != nil {
		log.Fatal("parse redis url : ", err)
	}
	redisPassword, _ := u.User.Password()

	// create redis client
	redisDB := 0
	redisClient := redis.NewClient(&redis.Options{
		Addr:     u.Host,
		Password: redisPassword,
		DB:       redisDB,
	})

	// ping to redis server
	if err = redisClient.Ping().Err(); err != nil {
		_ = redisClient.Close()
		return nil, fmt.Errorf("ping to redis : %v", err)
	}

	return redisClient, nil
}

func main() {
	var (
		consumerKey    = os.Getenv("TWITTER_CONSUMER_KEY")
		consumerSecret = os.Getenv("TWITTER_CONSUMER_SECRET")
		accessToken    = os.Getenv("TWITTER_ACCESS_TOKEN")
		accessSecret   = os.Getenv("TWITTER_ACCESS_TOKEN_SECRET")
		redisUrl       = os.Getenv("REDIS_URL")
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

	// setup redis client
	redisClient, err := setupRedis(redisUrl)
	if err != nil {
		log.Panic("failed to setup redis client :", err)
	} else if redisClient != nil {
		log.Println("redis is enabled")
		defer redisClient.Close()
	}

	// create twitter auth context
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)

	// setup scheduler
	sche := htask.NewScheduler(&wg, 0)
	defer sche.Close()

	for _, t := range flagTargets {
		// create task
		task, err := setupTask(ctx, config, token, redisClient, t, interval, timeout, fallback)
		if err != nil {
			log.Panic("failed to create task :", err)
		}

		// start task.
		err = task.Start(ctx, sche)
		if err != nil {
			log.Panic("failed to start task", err)
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

type idStore struct {
	client   *redis.Client
	targetId string
	latestId int64
}

func createIdStore(client *redis.Client, targetId int64) (*idStore, error) {
	targetIdStr := strconv.FormatInt(targetId, 10)
	var latestId int64
	if client != nil {
		// get latest id from redis store
		var err error
		if latestId, err = client.Get(targetIdStr).Int64(); err != nil && err != redis.Nil {
			return nil, err
		}
	}
	return &idStore{
		client:   client,
		targetId: targetIdStr,
		latestId: latestId,
	}, nil
}

func (l *idStore) update(latestId int64) error {
	l.latestId = latestId
	if l.client == nil {
		return nil
	}
	return l.client.Set(l.targetId, latestId, 0).Err()
}

func (l *idStore) get() int64 {
	return l.latestId
}

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

func (t *Task) twitterClient(ctx context.Context) *twitter.Client {
	httpClient := t.oauthConfig.Client(ctx, t.oauthToken)
	return twitter.NewClient(httpClient)
}

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
