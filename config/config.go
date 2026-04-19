package config

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	LLM      LLMConfig
	Auth     AuthConfig
	RateLimit RateLimitConfig
}

type ServerConfig struct {
	Port string
	Mode string
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type LLMConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

type AuthConfig struct {
	AppIDs    map[string]string
}

type RateLimitConfig struct {
	Global int
	App    int
	IP     int
}

var (
	cfg  *Config
	once sync.Once
)

func Load() *Config {
	once.Do(func() {
		godotenv.Load()
		cfg = &Config{
			Server: ServerConfig{
				Port: getEnv("PORT", "8080"),
				Mode: getEnv("MODE", "debug"),
			},
			Database: DatabaseConfig{
				URL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/address_parse"),
			},
			Redis: RedisConfig{
				URL: getEnv("REDIS_URL", "redis://localhost:6379"),
			},
			LLM: LLMConfig{
				APIKey:  getEnv("DASHSCOPE_API_KEY", ""),
				Model:   getEnv("LLM_MODEL", "qwen-turbo"),
				BaseURL: getEnv("LLM_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
			},
			RateLimit: RateLimitConfig{
				Global: getEnvInt("RATE_LIMIT_GLOBAL", 5000),
				App:    getEnvInt("RATE_LIMIT_APP", 500),
				IP:     getEnvInt("RATE_LIMIT_IP", 1000),
			},
		}

		appIDs := getEnv("APP_IDS", "")
		appSecrets := getEnv("APP_SECRETS", "")
		if appIDs != "" && appSecrets != "" {
			ids := strings.Split(appIDs, ",")
			secrets := strings.Split(appSecrets, ",")
			cfg.Auth.AppIDs = make(map[string]string)
			for i, id := range ids {
				if i < len(secrets) {
					cfg.Auth.AppIDs[strings.TrimSpace(id)] = strings.TrimSpace(secrets[i])
				}
			}
		}
	})
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func Get() *Config {
	if cfg == nil {
		return Load()
	}
	return cfg
}
