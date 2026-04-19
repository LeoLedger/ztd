package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/internal/handler"
	"github.com/your-name/address-parse/internal/repository"
	"github.com/your-name/address-parse/internal/service"
)

func main() {
	cfg := config.Load()

	redisClient, err := newRedisClient(cfg.Redis.URL)
	if err != nil {
		log.Printf("Warning: Redis unavailable, using in-memory fallback: %v", err)
		redisClient = nil
	} else {
		defer func() { _ = redisClient.Close() }()
	}

	historyRepo, err := repository.NewHistoryRepository(cfg.Database.URL)
	if err != nil {
		log.Printf("Warning: Database unavailable, history logging disabled: %v", err)
		historyRepo = nil
	} else {
		defer func() { _ = historyRepo.Close() }()
	}

	parserService := service.NewParserService(cfg, redisClient)

	h := handler.NewAddressHandler(parserService, historyRepo)

	r := handler.SetupRouter(h, cfg, redisClient)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}

func newRedisClient(redisURL string) (*redis.Client, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL not configured")
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}
	return client, nil
}

