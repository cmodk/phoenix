package app

import (
	"github.com/go-redis/redis"
)

func ConnectRedis(config string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	return rdb, nil
}
