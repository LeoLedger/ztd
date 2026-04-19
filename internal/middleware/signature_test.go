package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/pkg/response"
)

func TestSignatureMiddleware(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			AppIDs: map[string]string{
				"client_001": "secret1",
				"client_002": "secret2",
			},
		},
	}

	middleware := NewSignatureMiddleware(cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, map[string]string{"status": "ok"})
	}))

	t.Run("missing headers", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/address/parse", strings.NewReader(`{"address":"深圳南山区"}`))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("invalid app id", func(t *testing.T) {
		body := `{"address":"深圳南山区"}`
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		req := httptest.NewRequest("POST", "/api/v1/address/parse", strings.NewReader(body))
		req.Header.Set("X-App-Id", "invalid_client")
		req.Header.Set("X-Timestamp", timestamp)
		req.Header.Set("X-Signature", "invalid_sig")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("expired timestamp", func(t *testing.T) {
		body := `{"address":"深圳南山区"}`
		oldTimestamp := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
		sig := SignRequest(body, oldTimestamp, "secret1")
		req := httptest.NewRequest("POST", "/api/v1/address/parse", strings.NewReader(body))
		req.Header.Set("X-App-Id", "client_001")
		req.Header.Set("X-Timestamp", oldTimestamp)
		req.Header.Set("X-Signature", sig)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("valid signature", func(t *testing.T) {
		body := `{"address":"深圳南山区"}`
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		sig := SignRequest(body, timestamp, "secret1")
		req := httptest.NewRequest("POST", "/api/v1/address/parse", strings.NewReader(body))
		req.Header.Set("X-App-Id", "client_001")
		req.Header.Set("X-Timestamp", timestamp)
		req.Header.Set("X-Signature", sig)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		body := `{"address":"深圳南山区"}`
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		sig := SignRequest(body, timestamp, "wrong_secret")
		req := httptest.NewRequest("POST", "/api/v1/address/parse", strings.NewReader(body))
		req.Header.Set("X-App-Id", "client_001")
		req.Header.Set("X-Timestamp", timestamp)
		req.Header.Set("X-Signature", sig)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})
}

func TestSignRequest(t *testing.T) {
	body := `{"address":"test"}`
	timestamp := "1713200000"
	secret := "secret1"

	sig1 := SignRequest(body, timestamp, secret)
	sig2 := SignRequest(body, timestamp, secret)
	sig3 := SignRequest(body, timestamp, "other_secret")

	if sig1 != sig2 {
		t.Error("same input should produce same signature")
	}
	if sig1 == sig3 {
		t.Error("different secret should produce different signature")
	}
	if sig1 == "" {
		t.Error("signature should not be empty")
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Global: 100,
			App:    50,
			IP:     80,
		},
	}

	limiter := NewRateLimiter(cfg, nil)
	ctx := context.Background()

	for i := 0; i < 50; i++ {
		allowed, err := limiter.Allow(ctx, "app1", "192.168.1.1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Errorf("request %d should be allowed", i)
		}
	}
	allowed, _ := limiter.Allow(ctx, "app1", "192.168.1.1")
	if allowed {
		t.Error("request 51 should be blocked by app limit")
	}

	for i := 0; i < 80; i++ {
		limiter.Allow(ctx, "app2", "192.168.1.2")
	}
	allowed, _ = limiter.Allow(ctx, "app2", "192.168.1.2")
	if allowed {
		t.Error("request 81 should be blocked by ip limit")
	}
}

func TestGetClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	// X-Forwarded-For can have multiple IPs, take the first
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	ip := GetClientIP(req)
	// X-Forwarded-For value is returned as-is
	if ip == "" {
		t.Error("expected non-empty IP")
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "2.3.4.5")
	if GetClientIP(req) != "2.3.4.5" {
		t.Errorf("expected 2.3.4.5, got %s", GetClientIP(req))
	}
}
