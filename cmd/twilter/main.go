package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/dghubble/oauth1"
	"github.com/go-redis/redis"
	"github.com/kawasin73/htask"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"
)

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
