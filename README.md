# Twilter

Twilter is **Tw**it**ter** f**ilter**ing service.

`twilter` command is a daemon which **Monitors** and **Filters** tweets of a targeted user and **Retweet** filtered tweets.

`github.com/kawasin73/twilter` is library.
`github.com/kawasin73/twilter/cmd/twilter` is command using this library.

**Targeted Users**

- I only want to see someone's photo tweets at TimeLine, but not text tweets.
- I only want to see someone's Retweets at Timeline, but not other ordinary tweets.

## Example

```bash
$ export TWITTER_CONSUMER_KEY=xxxxxxxxxxxxxxxxxx
$ export TWITTER_CONSUMER_SECRET=xxxxxxxxxxxxxxxxxx
$ export TWITTER_ACCESS_TOKEN=xxxxxxxxxxxxxxxxxx
$ export TWITTER_ACCESS_TOKEN_SECRET=xxxxxxxxxxxxxxxxxx
$ export REDIS_URL=redis://user:password@host_name:6379
# Monitors photo tweets of @kawasin73 and original (not Retweet) photo tweets \
#    of @TwitterAPI and Retweets by @TwitterAPI
$ twilter -target "kawasin73:photo" -target "TwitterAPI:and(photo,not(rt))/rt"
```

## How to Use

You needs following things for Twitler.

- Twitter Account which you use
- Dummy Twitter Account (this should be private account)
- Twitter Developer Account
- Server in which daemon can run.
- Redis (optional)
- `twilter` binary

### Installation

```bash
$ go get github.com/kawasin73/twilter/cmd/twilter
```

`twilter` also supply docker image at https://hub.docker.com/r/kawasin73/twilter

```bash
$ docker pull kawasin73/twilter
```

### Setup steps

1. Create Twitter Developer Account (this may be the hardest step for you). (https://developer.twitter.com/en/apply-for-access)
2. Create Twitter App by your developer account (https://developer.twitter.com/en/docs/basics/apps/overview)
3. Create a dummy account and follow the dummy account from your account.
4. Generate `Consumer API Keys` of developer account and `Access Token + Secret` of dummy account.
    - if your dummy account is same with developer account, you can generate in dashboard (https://developer.twitter.com/en/docs/basics/authentication/guides/access-tokens.html)
    - if your dummy account is not your developer account, [twitter-auth](https://github.com/k0kubun/twitter-auth) tool will help you.
5. Run `twilter` command by daemon mode (by using initd, systemd, kubernetes etc...).
6. If you want to shutdown `twilter` then send `SIGINT` signal (Ctrl + C)

### Redis usage

`twilter` will save latest tweet id which twilter have monitored to **Redis** only when you set `REDIS_URL` environment variable.

`twilter` will start monitoring from tweet `fallback` minutes before start time if you do not set `REDIS_URL`.

## Variables

```
$ twilter -h
Usage of /usr/local/bin/twilter:
  -fallback int
    	start filtering tweets fallback minutes ago if no checkpoint (minutes) (default 10)
  -interval int
    	interval between monitoring (minutes) (default 10)
  -target value
    	list of targets. target format = "<screen_name>:<filter>[/<filter>]"  filter format = "<filter_name>[(<attribute>[,<attribute>])]"
  -timeout int
    	timeout for each monitoring + retweet loop (minutes) (default 5)
```

## Filters

- `rt` : filters only Retweets.
- `qt` : filters only Quoted Tweets.
- `photo` : filters only tweets that include photo.
- `video` : filters only tweets that include video.
- `not(<filter>)` : filters only tweets that `<filter>` does not match.
- `and(<filter>[,<filter>[,...]])` : filters only tweets that all filters match.
- `or(<filter>[,<filter>[,...]])` : filters only tweets that at least one filter match.

### TODOs

- [ ] `keyword(<string>)` : filters only tweets that include the keyword.
- [ ] `hashtag(<string)` : filters only tweets that include the hashtag.
- [ ] `link` : filters only tweets that include url link.

## Dependencies

`twilter` uses following packages

- `github.com/dghubble/go-twitter` : Very simple Twitter client library
- `github.com/dghubble/oauth1` : Builds `*http.Client` which handles OAuth1
- `github.com/go-redis/redis` : Nice Redis client library
- `github.com/kawasin73/htask` : High Scalable In-memory task scheduler using Min Heap and less goroutines

## LICENSE

MIT
