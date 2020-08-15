package redis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
)

var (
	Client *redis.Client
)

func init()  {
	Client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "&X69xTT7*wmjYD&XT2dQ61YxM%",
		DB:       0,
	})
	cmd := Client.Ping(context.Background())
	if res := cmd.Val(); res != "PONG" {
		log.Fatalf("Could not ping the redis server: %v", cmd.Err())
	}
}
