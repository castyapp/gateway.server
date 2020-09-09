package redis

import (
	"context"
	"fmt"
	"github.com/CastyLab/gateway.server/config"
	"github.com/go-redis/redis/v8"
	"log"
)

var (
	Client *redis.Client
)

func Configure() error {
	var (
		host     = config.Map.Secrets.Redis.Host
		port     = config.Map.Secrets.Redis.Port
		password = config.Map.Secrets.Redis.Pass
	)
	Client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       0,
	})
	cmd := Client.Ping(context.Background())
	if res := cmd.Val(); res != "PONG" {
		log.Fatalf("Could not ping the redis server: %v", cmd.Err())
	}
	return nil
}
