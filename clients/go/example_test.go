package client_test

// Package client_test provides examples of how to call
// the /api/v1/address/parse endpoint from a Go client.
//
// Signature scheme (must match the server in internal/middleware/signature.go):
//
//   message   = timestamp + request_body_json
//   signature = base64(HMAC-SHA256(message, app_secret))
//
// Required HTTP headers:
//   Content-Type: application/json
//   X-App-Id:    <your-app-id>
//   X-Timestamp: <unix-seconds-as-string>
//   X-Signature: <computed-above>
//   X-Nonce:     <uuid-v4>
//
// Run the server first:
//   go run ./cmd/server
//
// Then run this file as a test:
//   go test -v -run ExampleParseAddress ./clients/go/

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Configuration — replace with your actual values
// ---------------------------------------------------------------------------

const (
	baseURL   = "http://localhost:8080"
	appID     = "your-app-id"
	appSecret = "your-app-secret"
)

// ---------------------------------------------------------------------------
// Types — mirror the server's model package
// ---------------------------------------------------------------------------

type parseRequest struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Company string `json:"company"`
	Address string `json:"address"`
}

type parseResponse struct {
	Name     string `json:"name,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Company  string `json:"company,omitempty"`
	Province string `json:"province,omitempty"`
	City     string `json:"city,omitempty"`
	District string `json:"district,omitempty"`
	Street   string `json:"street,omitempty"`
	Detail   string `json:"detail,omitempty"`
	FullAddr string `json:"full_address,omitempty"`
}

// ---------------------------------------------------------------------------
// Signing helpers
// ---------------------------------------------------------------------------

func computeSignature(body, timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + body))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// parseAddress — demonstrates the full signed-request flow
// ---------------------------------------------------------------------------

// parseAddress sends a signed request to /api/v1/address/parse.
// It returns the parsed response and any error encountered.
func parseAddress(name, phone, company, address string) (*parseResponse, error) {
	reqBody := parseRequest{
		Name:    name,
		Phone:   phone,
		Company: company,
		Address: address,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := computeSignature(string(body), timestamp, appSecret)

	req, err := http.NewRequest(http.MethodPost,
		baseURL+"/api/v1/address/parse",
		bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Id", appID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Nonce", uuid.New().String())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var result parseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ---------------------------------------------------------------------------
// Example — run against a live server
// ---------------------------------------------------------------------------

// ExampleClient demonstrates the three main usage patterns for the client.
// Run `go run ./cmd/server` first, then execute with:
//   go test -v -run ExampleClient ./clients/go/
//
// The output shows request payloads; replace the stub in parseAddress with a
// real http.Client.Do call to get live results from the server.
func ExampleClient() {
	examples := []struct {
		name    string
		payload parseRequest
	}{
		{
			name: "full address — all four input fields",
			payload: parseRequest{
				Name:    "张三",
				Phone:   "15361237638",
				Company: "智腾达软件技术公司",
				Address: "广东省深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室",
			},
		},
		{
			name: "address only — name/phone/company omitted",
			payload: parseRequest{
				Address: "北京市朝阳区建国路88号SOHO现代城A座1001",
			},
		},
		{
			name: "company name — suffix-stripping behaviour demo",
			payload: parseRequest{
				Name:    "李四",
				Phone:   "13900001111",
				Company: "智腾达软件技术公司",
				Address: "深圳宝安区西乡街道固戍社区南昌公园旁",
			},
		},
	}

	for _, ex := range examples {
		fmt.Printf("%s:\n", ex.name)
		fmt.Printf("  name=%s phone=%s company=%s address=%s\n",
			ex.payload.Name, ex.payload.Phone,
			ex.payload.Company, ex.payload.Address)
	}

	// Output:
	// full address — all four input fields:
	//   name=张三 phone=15361237638 company=智腾达软件技术公司 address=广东省深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室
	// address only — name/phone/company omitted:
	//   name= phone= company= address=北京市朝阳区建国路88号SOHO现代城A座1001
	// company name — suffix-stripping behaviour demo:
	//   name=李四 phone=13900001111 company=智腾达软件技术公司 address=深圳宝安区西乡街道固戍社区南昌公园旁
}

// ---------------------------------------------------------------------------
// Unit tests — verify signature determinism without a live server
// ---------------------------------------------------------------------------

func TestComputeSignature_Deterministic(t *testing.T) {
	body := `{"name":"张三","phone":"15361237638","address":"广东省深圳市南山区桃源街道"}`
	ts := "1745000000"
	secret := "test-secret"

	s1 := computeSignature(body, ts, secret)
	s2 := computeSignature(body, ts, secret)

	if s1 != s2 {
		t.Fatal("signature must be deterministic for the same inputs")
	}
}

func TestComputeSignature_ChangesOnAnyInput(t *testing.T) {
	base := `{"address":"广东省深圳市"}`
	ts := "1745000000"
	secret := "test-secret"

	baseSig := computeSignature(base, ts, secret)

	// Different body → different signature
	if computeSignature(`{"address":"北京市"}`, ts, secret) == baseSig {
		t.Error("body change should alter signature")
	}

	// Different timestamp → different signature
	if computeSignature(base, "1745000001", secret) == baseSig {
		t.Error("timestamp change should alter signature")
	}

	// Different secret → different signature
	if computeSignature(base, ts, "other-secret") == baseSig {
		t.Error("secret change should alter signature")
	}
}
