package redis

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"os"
)

var (
	Client *redis.Client
)

func init()  {

	var (
		host     = os.Getenv("REDIS_HOST")
		port     = os.Getenv("REDIS_PORT")
		password = os.Getenv("REDIS_PASS")
	)

	Client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       0,
	})

	cmd := Client.Ping(context.Background())
	if res := cmd.Val(); res != "PONG" {
		log.Fatalf("Could not ping the redis server: %v", cmd.Err())
	}

}
