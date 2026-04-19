package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/pkg/response"
)

const (
	HeaderAppID    = "X-App-Id"
	HeaderTimestamp = "X-Timestamp"
	HeaderSignature = "X-Signature"
	HeaderNonce    = "X-Nonce"

	TimestampValidity = 5 * time.Minute
	NonceTTL          = 10 * time.Minute
	NonceCacheMax     = 100_000
)

// nonceStore abstracts the underlying nonce storage backend.
type nonceStore interface {
	exists(nonce string) (bool, error)
}

// redisNonceStore persists nonces in Redis with atomic SET NX semantics.
type redisNonceStore struct {
	client *redis.Client
	ttl    time.Duration
}

func newRedisNonceStore(client *redis.Client) *redisNonceStore {
	return &redisNonceStore{client: client, ttl: NonceTTL}
}

func (s *redisNonceStore) exists(nonce string) (bool, error) {
	key := "nonce:" + nonce
	ok, err := s.client.SetNX(context.Background(), key, "1", s.ttl).Result()
	if err != nil {
		return false, err
	}
	return !ok, nil // SetNX returns true when key was already set
}

// memNonceStore is a thread-safe in-memory fallback when Redis is unavailable.
type memNonceStore struct {
	mu    sync.RWMutex
	nonce map[string]struct{}
	keys  []string
}

func newMemNonceStore() *memNonceStore {
	return &memNonceStore{nonce: make(map[string]struct{})}
}

func (s *memNonceStore) exists(nonce string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.nonce[nonce]; ok {
		return true, nil
	}
	s.nonce[nonce] = struct{}{}
	s.keys = append(s.keys, nonce)

	if len(s.nonce) > NonceCacheMax {
		evict := s.keys[:len(s.keys)/2]
		s.keys = s.keys[len(evict):]
		for _, k := range evict {
			delete(s.nonce, k)
		}
	}
	return false, nil
}

type signatureMiddleware struct {
	appIDs map[string]string
	store  nonceStore
}

func NewSignatureMiddleware(cfg *config.Config, redisClient *redis.Client) func(http.Handler) http.Handler {
	var store nonceStore
	if redisClient != nil {
		store = newRedisNonceStore(redisClient)
	} else {
		store = newMemNonceStore()
	}
	return (&signatureMiddleware{
		appIDs: cfg.Auth.AppIDs,
		store:  store,
	}).Middleware()
}

func (m *signatureMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			appID := r.Header.Get(HeaderAppID)
			timestampStr := r.Header.Get(HeaderTimestamp)
			signature := r.Header.Get(HeaderSignature)

			fmt.Printf("[DEBUG] appID=%q timestamp=%q signature=%q\n", appID, timestampStr, signature)

			if appID == "" || timestampStr == "" || signature == "" {
				response.Unauthorized(w, "missing signature headers")
				return
			}

			secret, ok := m.appIDs[appID]
			if !ok {
				fmt.Printf("[DEBUG] appID=%q not found in map, available: %v\n", appID, m.appIDs)
				response.Unauthorized(w, "invalid app id")
				return
			}

			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				response.Unauthorized(w, "invalid timestamp")
				return
			}

			requestTime := time.Unix(timestamp, 0)
			if time.Since(requestTime) > TimestampValidity || time.Until(requestTime) > TimestampValidity {
				response.Unauthorized(w, "timestamp expired or invalid")
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				response.InternalError(w, "failed to read body")
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			message := timestampStr + string(body)
			expectedSig := computeHMAC(message, secret)

			fmt.Printf("[DEBUG] body=%q expectedSig=%q\n", string(body), expectedSig)

			if subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSig)) != 1 {
				response.Unauthorized(w, "signature mismatch")
				return
			}

			nonce := r.Header.Get(HeaderNonce)
			if nonce != "" {
				dupe, err := m.store.exists(nonce)
				if err != nil {
					response.InternalError(w, "nonce check failed")
					return
				}
				if dupe {
					response.Unauthorized(w, "nonce reused")
					return
				}
			}

			r.Header.Set("X-App-Id", appID)
			next.ServeHTTP(w, r)
		})
	}
}

func computeHMAC(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func SignRequest(body, timestamp, appSecret string) string {
	return computeHMAC(timestamp+body, appSecret)
}
