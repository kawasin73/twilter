package main

import (
	"github.com/go-redis/redis"
	"strconv"
)

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

	// store new latestId to redis
	return l.client.Set(l.targetId, latestId, 0).Err()
}

func (l *idStore) get() int64 {
	return l.latestId
}
