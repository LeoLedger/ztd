package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	godotenv.Load()
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("parse redis URL failed: %v", err)
	}
	client := redis.NewClient(opt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis connect failed: %v", err)
	}

	keys, err := client.Keys(ctx, "rl:*").Result()
	if err != nil {
		log.Fatalf(" KEYS rl:* failed: %v", err)
	}

	if len(keys) == 0 {
		fmt.Println("No rate limit keys found.")
		return
	}

	if err := client.Del(ctx, keys...).Err(); err != nil {
		log.Fatalf("DEL failed: %v", err)
	}

	fmt.Printf("Deleted %d keys: %v\n", len(keys), keys)
}
