package redis

import (
	"context"
	"log"

	"github.com/castyapp/gateway.server/config"
	"github.com/go-redis/redis/v8"
)

var (
	Client *redis.Client
)

func Configure() error {

	if config.Map.Redis.Cluster {
		Client = redis.NewFailoverClient(&redis.FailoverOptions{
			SentinelAddrs:    config.Map.Redis.Sentinels,
			Password:         config.Map.Redis.Pass,
			SentinelPassword: config.Map.Redis.SentinelPass,
			MasterName:       config.Map.Redis.MasterName,
			DB:               0,
		})
	} else {
		Client = redis.NewClient(&redis.Options{
			Addr:     config.Map.Redis.Addr,
			Password: config.Map.Redis.Pass,
		})
	}

	cmd := Client.Ping(context.Background())
	if res := cmd.Val(); res != "PONG" {

		if config.Map.Redis.Cluster {
			log.Println("SentinelAddrs: ", config.Map.Redis.Sentinels)
		} else {
			log.Println("Addr: ", config.Map.Redis.Addr)
		}

		log.Fatalf("Could not ping the redis server: %v", cmd.Err())
	}
	return nil
}

func Close() error {
	return Client.Close()
}
