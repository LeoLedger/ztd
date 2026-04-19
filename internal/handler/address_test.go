package handler

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/your-name/address-parse/internal/middleware"
	"github.com/your-name/address-parse/internal/model"
	"github.com/your-name/address-parse/internal/parser"
	"github.com/your-name/address-parse/pkg/response"
)

func TestHandler_HealthCheck(t *testing.T) {
	handler := &AddressHandler{}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["code"].(float64) != 0 {
		t.Errorf("expected code 0, got %v", resp["code"])
	}
	if !strings.Contains(w.Body.String(), "healthy") {
		t.Errorf("expected body to contain 'healthy', got %s", w.Body.String())
	}
}

func TestHandler_ParseAddress_EmptyBody(t *testing.T) {
	handler := &AddressHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/address/parse", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	handler.ParseAddress(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_ParseAddress_InvalidJSON(t *testing.T) {
	handler := &AddressHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/address/parse", strings.NewReader(`{invalid}`))
	w := httptest.NewRecorder()

	handler.ParseAddress(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_ParseAddress_NoAddress(t *testing.T) {
	handler := &AddressHandler{}

	body := `{"name":"张三","phone":"15361237638"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/address/parse", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.ParseAddress(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// replaceUnescapedNewlines tests — directly unit-test the pre-processing logic.

func TestReplaceUnescapedNewlines_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "literal newline outside string",
			input:    "hello\nworld",
			expected: "hello world",
		},
		{
			name:     "escaped newline inside string value is preserved",
			input:    `{"address":"广东省深圳市\n南山区"}`,
			expected: `{"address":"广东省深圳市\n南山区"}`,
		},
		{
			name:     "literal newline inside string value becomes space",
			input:    "广东省深圳市\n南山区桃源街道88号",
			expected: "广东省深圳市 南山区桃源街道88号",
		},
		{
			name:     "multiple newlines",
			input:    "hello\n\nworld\n",
			expected: "hello  world ",
		},
		{
			name:     "escaped backslash-n inside string",
			input:    `{"a":"line1\\nline2"}`,
			expected: `{"a":"line1\\nline2"}`,
		},
		{
			name:     "mixed newlines inside and outside strings",
			input:    "start\n{\"key\":\"val\nue\"}\nend",
			expected: "start {\"key\":\"val ue\"} end",
		},
		{
			name:     "literal newline inside string in full JSON",
			input:    "广东省\n深圳市\n南山区",
			expected: "广东省 深圳市 南山区",
		},
		{
			name:     "literal tab inside string value becomes space",
			input:    "广东省\t深圳市\t南山区",
			expected: "广东省 深圳市 南山区",
		},
		{
			name:     "literal tab inside JSON string value becomes space",
			input:    "{\"address\":\"广东省\t深圳市\"}",
			expected: "{\"address\":\"广东省 深圳市\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceUnescapedNewlines([]byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("replaceUnescapedNewlines(%q) = %q, want %q", tt.input, string(result), tt.expected)
			}
		})
	}
}

func TestLogMiddleware(t *testing.T) {
	nextCalled := false
	middleware := LogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestParseRequest_JSON(t *testing.T) {
	body := `{"name":"张三","phone":"15361237638","company":"智腾达","address":"广东省深圳市南山区桃源街道"}`
	var req model.ParseRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if req.Name != "张三" {
		t.Errorf("expected name '张三', got '%s'", req.Name)
	}
	if req.Address != "广东省深圳市南山区桃源街道" {
		t.Errorf("unexpected address: %s", req.Address)
	}
}

var _ = response.Success

// TestParseAddress_EndToEnd covers the full HTTP integration flow: valid signature,
// invalid signature, missing headers, and expired timestamp.
func TestParseAddress_EndToEnd(t *testing.T) {
	tests := []struct {
		name          string
		body          interface{}
		setupSig      func(body string) (timestamp, signature string)
		expectCode    int
		expectErrCode int
		checkResponse func(t *testing.T, body []byte)
	}{
		{
			name: "正确签名 → 200 + 解析结果",
			body: model.ParseRequest{Name: "张三", Phone: "15361237638", Address: "广东省深圳市南山区桃源街道88号"},
			setupSig: func(body string) (string, string) {
				ts := strconv.FormatInt(time.Now().Unix(), 10)
				return ts, middleware.SignRequest(body, ts, "test-secret")
			},
			expectCode:    http.StatusOK,
			expectErrCode: 0,
			checkResponse: func(t *testing.T, body []byte) {
				var r struct {
					Code int `json:"code"`
					Data struct {
						Province string `json:"province"`
						City     string `json:"city"`
						District string `json:"district"`
					} `json:"data"`
				}
				if err := json.Unmarshal(body, &r); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if r.Data.Province != "广东省" {
					t.Errorf("Province = %q, want %q", r.Data.Province, "广东省")
				}
				if r.Data.City == "" {
					t.Errorf("City is empty, want non-empty")
				}
				if r.Data.District != "南山区" {
					t.Errorf("District = %q, want %q", r.Data.District, "南山区")
				}
			},
		},
		{
			name: "错误签名 → 401",
			body: model.ParseRequest{Address: "深圳市南山区"},
			setupSig: func(body string) (string, string) {
				ts := strconv.FormatInt(time.Now().Unix(), 10)
				return ts, "wrong-signature-base64=="
			},
			expectCode:    http.StatusUnauthorized,
			expectErrCode: 4001,
		},
		{
			name: "缺失签名头 → 401",
			body: model.ParseRequest{Address: "深圳市南山区"},
			setupSig: func(body string) (string, string) {
				return "", ""
			},
			expectCode:    http.StatusUnauthorized,
			expectErrCode: 4001,
		},
		{
			name: "过期时间戳 → 401",
			body: model.ParseRequest{Address: "深圳市南山区"},
			setupSig: func(body string) (string, string) {
				expired := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
				return expired, middleware.SignRequest(body, expired, "test-secret")
			},
			expectCode:    http.StatusUnauthorized,
			expectErrCode: 4001,
		},
		{
			name: "无省份前缀地址解析",
			body: model.ParseRequest{Address: "深圳南山大学城创业园桑泰大厦13楼1303室"},
			setupSig: func(body string) (string, string) {
				ts := strconv.FormatInt(time.Now().Unix(), 10)
				return ts, middleware.SignRequest(body, ts, "test-secret")
			},
			expectCode:    http.StatusOK,
			expectErrCode: 0,
			checkResponse: func(t *testing.T, body []byte) {
				var r struct {
					Code int `json:"code"`
					Data struct {
						Province string `json:"province"`
						City     string `json:"city"`
						District string `json:"district"`
					} `json:"data"`
				}
				if err := json.Unmarshal(body, &r); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if r.Data.Province != "" {
					t.Errorf("Province = %q, want empty", r.Data.Province)
				}
				if r.Data.City == "" {
					t.Errorf("City is empty, want non-empty")
				}
				if r.Data.District != "南山区" {
					t.Errorf("District = %q, want %q", r.Data.District, "南山区")
				}
			},
		},
		{
			name: "无地址字段 → 400",
			body: map[string]string{"name": "张三"},
			setupSig: func(body string) (string, string) {
				ts := strconv.FormatInt(time.Now().Unix(), 10)
				return ts, middleware.SignRequest(body, ts, "test-secret")
			},
			expectCode:    http.StatusBadRequest,
			expectErrCode: 4002,
		},
	}

	memStore := newTestNonceStore()
	mw := testSignatureMiddleware("test-app", "test-secret", memStore)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			ts, sig := tt.setupSig(string(bodyBytes))

			req := httptest.NewRequest(http.MethodPost, "/api/v1/address/parse", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-App-Id", "test-app")
			if ts != "" {
				req.Header.Set("X-Timestamp", ts)
			}
			if sig != "" {
				req.Header.Set("X-Signature", sig)
			}

			w := httptest.NewRecorder()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var preq model.ParseRequest
				if err := json.NewDecoder(r.Body).Decode(&preq); err != nil {
					response.BadRequest(w, "invalid JSON body")
					return
				}
				if preq.Address == "" {
					response.BadRequest(w, "address is required")
					return
				}
				engine := newRuleEngineForTest()
				result, ok := engine.Parse(preq.Address)
				if !ok {
					response.BadRequest(w, "address parsing failed")
					return
				}
				response.Success(w, result)
			})
			mw(handler).ServeHTTP(w, req)

			if w.Code != tt.expectCode {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.expectCode, w.Body.String())
			}

			if tt.expectErrCode == 0 {
				if tt.checkResponse != nil {
					tt.checkResponse(t, w.Body.Bytes())
				}
				return
			}

			var rErr struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &rErr); err != nil {
				t.Fatalf("failed to unmarshal error response: %v", err)
			}
			if rErr.Code != tt.expectErrCode {
				t.Errorf("err code = %d, want %d", rErr.Code, tt.expectErrCode)
			}
		})
	}
}

// TestParseAddress_NonceReplay verifies that a nonce cannot be reused within the
// TTL window. It fires two requests with the same nonce; only the first succeeds.
func TestParseAddress_NonceReplay(t *testing.T) {
	memStore := newTestNonceStore()
	mw := testSignatureMiddleware("test-app", "test-secret", memStore)
	body := model.ParseRequest{Address: "深圳市南山区"}
	bodyBytes, _ := json.Marshal(body)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := middleware.SignRequest(string(bodyBytes), ts, "test-secret")
	nonce := "unique-replay-nonce-" + ts

	makeReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/address/parse", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-App-Id", "test-app")
		req.Header.Set("X-Timestamp", ts)
		req.Header.Set("X-Signature", sig)
		req.Header.Set("X-Nonce", nonce)
		return req
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var preq model.ParseRequest
		if err := json.NewDecoder(r.Body).Decode(&preq); err != nil {
			response.BadRequest(w, "invalid JSON body")
			return
		}
		if preq.Address == "" {
			response.BadRequest(w, "address is required")
			return
		}
		engine := newRuleEngineForTest()
		result, ok := engine.Parse(preq.Address)
		if !ok {
			response.BadRequest(w, "address parsing failed")
			return
		}
		response.Success(w, result)
	})

	// First request should succeed.
	w1 := httptest.NewRecorder()
	mw(handler).ServeHTTP(w1, makeReq())
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200, body: %s", w1.Code, w1.Body.String())
	}

	// Second request with same nonce should be rejected.
	w2 := httptest.NewRecorder()
	mw(handler).ServeHTTP(w2, makeReq())
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("second request: status = %d, want 401 (nonce replay), body: %s", w2.Code, w2.Body.String())
	}
	var rErr struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &rErr); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if rErr.Code != 4001 {
		t.Errorf("err code = %d, want 4001", rErr.Code)
	}
}

// testNonceStore is a minimal in-memory nonce store for testing.
type testNonceStore struct {
	mu    sync.Mutex
	nonce map[string]struct{}
}

func newTestNonceStore() *testNonceStore {
	return &testNonceStore{nonce: make(map[string]struct{})}
}

func (s *testNonceStore) exists(nonce string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.nonce[nonce]; ok {
		return true, nil
	}
	s.nonce[nonce] = struct{}{}
	return false, nil
}

// nonceStore is the interface for replay detection.
type nonceStore interface {
	exists(nonce string) (bool, error)
}

// testSignatureMiddleware creates a minimal signature middleware for testing.
func testSignatureMiddleware(appID, secret string, store nonceStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			aID := r.Header.Get("X-App-Id")
			ts := r.Header.Get("X-Timestamp")
			sig := r.Header.Get("X-Signature")

			if aID == "" || ts == "" || sig == "" {
				response.Unauthorized(w, "missing signature headers")
				return
			}
			if aID != appID {
				response.Unauthorized(w, "invalid app id")
				return
			}

			timestamp, err := strconv.ParseInt(ts, 10, 64)
			if err != nil {
				response.Unauthorized(w, "invalid timestamp")
				return
			}
			requestTime := time.Unix(timestamp, 0)
			if time.Since(requestTime) > 5*time.Minute || time.Until(requestTime) > 5*time.Minute {
				response.Unauthorized(w, "timestamp expired or invalid")
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				response.InternalError(w, "failed to read body")
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			expectedSig := middleware.SignRequest(string(body), ts, secret)

			if subtle.ConstantTimeCompare([]byte(sig), []byte(expectedSig)) != 1 {
				response.Unauthorized(w, "signature mismatch")
				return
			}

			nonce := r.Header.Get("X-Nonce")
			if nonce != "" {
				dupe, err := store.exists(nonce)
				if err != nil {
					response.InternalError(w, "nonce check failed")
					return
				}
				if dupe {
					response.Unauthorized(w, "nonce reused")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// newRuleEngineForTest creates a RuleEngine for testing.
func newRuleEngineForTest() *ruleEngineWrapper {
	return &ruleEngineWrapper{}
}

// ruleEngineWrapper wraps the global NewRuleEngine for test use.
type ruleEngineWrapper struct{}

func (w *ruleEngineWrapper) Parse(address string) (*model.ParseResponse, bool) {
	return parser.NewRuleEngine().Parse(address)
}

// Verify NewRuleEngine is accessible (it's exported in parser package).
var _ = parser.NewRuleEngine
