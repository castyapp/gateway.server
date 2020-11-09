package redis

import (
	"context"
	"log"

	"github.com/CastyLab/gateway.server/config"
	"github.com/go-redis/redis/v8"
)

var (
	Client *redis.Client
)

func Configure() error {
	Client = redis.NewFailoverClient(&redis.FailoverOptions{
		SentinelAddrs:    config.Map.Secrets.Redis.Sentinels,
		Password:         config.Map.Secrets.Redis.Pass,
		SentinelPassword: config.Map.Secrets.Redis.SentinelPass,
		MasterName:       config.Map.Secrets.Redis.MasterName,
		DB:               0,
	})
	cmd := Client.Ping(context.Background())
	if res := cmd.Val(); res != "PONG" {
		log.Println("SentinelAddrs: ", config.Map.Secrets.Redis.Sentinels)
		log.Fatalf("Could not ping the redis server: %v", cmd.Err())
	}
	return nil
}

func Close() error {
	return Client.Close()
}
