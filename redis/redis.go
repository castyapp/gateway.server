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

	if config.Map.Secrets.Redis.Replicaset {
		Client = redis.NewFailoverClient(&redis.FailoverOptions{
			SentinelAddrs:    config.Map.Secrets.Redis.Sentinels,
			Password:         config.Map.Secrets.Redis.Pass,
			SentinelPassword: config.Map.Secrets.Redis.SentinelPass,
			MasterName:       config.Map.Secrets.Redis.MasterName,
			DB:               0,
		})
	} else {
		Client = redis.NewClient(&redis.Options{
			Addr:     config.Map.Secrets.Redis.Addr,
			Password: config.Map.Secrets.Redis.Pass,
		})
	}

	cmd := Client.Ping(context.Background())
	if res := cmd.Val(); res != "PONG" {

		if config.Map.Secrets.Redis.Replicaset {
			log.Println("SentinelAddrs: ", config.Map.Secrets.Redis.Sentinels)
		} else {
			log.Println("Addr: ", config.Map.Secrets.Redis.Addr)
		}

		log.Fatalf("Could not ping the redis server: %v", cmd.Err())
	}
	return nil
}

func Close() error {
	return Client.Close()
}
