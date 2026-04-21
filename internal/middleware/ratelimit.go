package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/pkg/response"
)

// ratelimitLua is a Lua script that implements atomic token-bucket rate limiting.
// KEYS[1] = rate limit key (e.g. "rl:global", "rl:app:<id>", "rl:ip:<ip>")
// ARGV[1] = limit (max tokens)
// ARGV[2] = window in seconds (TTL for the key)
// ARGV[3] = current unix timestamp in seconds (used for window reset)
const ratelimitLua = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local count = redis.call('GET', key)
if count and tonumber(count) >= limit then
    return 0
end
count = redis.call('INCR', key)
if count == 1 then redis.call('EXPIRE', key, window) end
return 1
`

// RateLimiter provides per-key rate limiting backed by Redis.
// Falls back to an in-memory token bucket when Redis is unavailable.
type RateLimiter struct {
	globalLimit int
	appLimit    int
	ipLimit     int

	redis       *redis.Client
	script      *redis.Script
	redisAvail  bool

	// In-memory fallback
	memTokens     int
	memAppTokens  map[string]int
	memIPTokens   map[string]int
	memLastRefill time.Time
	memMu         sync.Mutex
}

// NewRateLimiter creates a RateLimiter using Redis when client is non-nil,
// otherwise falls back to an in-memory implementation.
func NewRateLimiter(cfg *config.Config, redisClient *redis.Client) *RateLimiter {
	rl := &RateLimiter{
		globalLimit: cfg.RateLimit.Global,
		appLimit:    cfg.RateLimit.App,
		ipLimit:     cfg.RateLimit.IP,
		redis:       redisClient,
		script:      redis.NewScript(ratelimitLua),
		redisAvail:  redisClient != nil,
		memTokens:   cfg.RateLimit.Global,
		memAppTokens: make(map[string]int),
		memIPTokens:  make(map[string]int),
		memLastRefill: time.Now(),
	}
	return rl
}

func (rl *RateLimiter) refillMemTokensLocked() {
	now := time.Now()
	elapsed := now.Sub(rl.memLastRefill)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.globalLimit) / 60.0)
	if tokensToAdd <= 0 {
		return
	}
	rl.memTokens = min(rl.globalLimit, rl.memTokens+tokensToAdd)
	for appID, t := range rl.memAppTokens {
		rl.memAppTokens[appID] = min(rl.appLimit, t+tokensToAdd)
	}
	for ip, t := range rl.memIPTokens {
		rl.memIPTokens[ip] = min(rl.ipLimit, t+tokensToAdd)
	}
	rl.memLastRefill = now
}

// Allow checks whether a request from the given appID and IP is permitted.
func (rl *RateLimiter) Allow(ctx context.Context, appID, ip string) (bool, error) {
	if rl.globalLimit == 0 && rl.appLimit == 0 && rl.ipLimit == 0 {
		return true, nil
	}
	if rl.redisAvail {
		return rl.allowRedis(ctx, appID, ip)
	}
	return rl.allowMem(appID, ip), nil
}

func (rl *RateLimiter) allowRedis(ctx context.Context, appID, ip string) (bool, error) {
	nowSec := time.Now().Unix()
	ttlSec := 60

	keys := []string{
		"rl:global",
		"rl:app:" + appID,
		"rl:ip:" + ip,
	}
	limits := []int{rl.globalLimit, rl.appLimit, rl.ipLimit}

	for i, key := range keys {
		result, err := rl.script.Run(ctx, rl.redis, []string{key},
			limits[i], ttlSec, nowSec,
		).Int()
		if err != nil {
			rl.redisAvail = false
			return rl.allowMem(appID, ip), nil
		}
		if result == 0 {
			return false, nil
		}
	}
	return true, nil
}

func (rl *RateLimiter) allowMem(appID, ip string) bool {
	if rl.globalLimit == 0 && rl.appLimit == 0 && rl.ipLimit == 0 {
		return true
	}
	rl.memMu.Lock()
	defer rl.memMu.Unlock()

	rl.refillMemTokensLocked()

	// Global
	if rl.memTokens <= 0 {
		return false
	}

	// App-level
	appTokens := rl.memAppTokens[appID]
	if appTokens >= rl.appLimit {
		return false
	}

	// IP-level
	ipTokens := rl.memIPTokens[ip]
	if ipTokens >= rl.ipLimit {
		return false
	}

	rl.memTokens--
	rl.memAppTokens[appID] = appTokens + 1
	rl.memIPTokens[ip] = ipTokens + 1
	return true
}

// Middleware returns an http.HandlerFunc that enforces rate limits.
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			appID := r.Header.Get("X-App-Id")
			ip := GetClientIP(r)
			if appID == "" {
				appID = "anonymous"
			}

			allowed, err := rl.Allow(r.Context(), appID, ip)
			if err != nil {
				response.InternalError(w, "rate limiter error")
				return
			}
			if !allowed {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.appLimit))
				w.Header().Set("Retry-After", "60")
				response.Error(w, http.StatusTooManyRequests, 429, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// LimiterMiddleware adapts *RateLimiter to the middleware signature used in handler.SetupRouter.
type LimiterMiddleware struct {
	limiter *RateLimiter
}

func NewLimiterMiddleware(cfg *config.Config, redisClient *redis.Client) func(http.Handler) http.Handler {
	return NewRateLimiter(cfg, redisClient).Middleware()
}
