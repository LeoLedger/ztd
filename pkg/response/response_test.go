package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSuccess(t *testing.T) {
	rr := httptest.NewRecorder()
	Success(rr, map[string]string{"key": "value"})

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != CodeSuccess {
		t.Errorf("Code = %d, want %d", resp.Code, CodeSuccess)
	}
	if resp.Message != "success" {
		t.Errorf("Message = %q, want %q", resp.Message, "success")
	}
}

func TestBadRequest(t *testing.T) {
	rr := httptest.NewRecorder()
	BadRequest(rr, "missing field")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != CodeParamInvalid {
		t.Errorf("Code = %d, want %d", resp.Code, CodeParamInvalid)
	}
}

func TestUnauthorized(t *testing.T) {
	rr := httptest.NewRecorder()
	Unauthorized(rr, "invalid signature")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != CodeSignatureInvalid {
		t.Errorf("Code = %d, want %d", resp.Code, CodeSignatureInvalid)
	}
}

func TestInternalError(t *testing.T) {
	rr := httptest.NewRecorder()
	InternalError(rr, "something went wrong")

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != CodeSystemError {
		t.Errorf("Code = %d, want %d", resp.Code, CodeSystemError)
	}
}

func TestError(t *testing.T) {
	rr := httptest.NewRecorder()
	Error(rr, http.StatusTooManyRequests, 429, "rate limit exceeded")

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != 429 {
		t.Errorf("Code = %d, want %d", resp.Code, 429)
	}
}

func TestJSONNoExtraSpaceInCJK(t *testing.T) {
	rr := httptest.NewRecorder()
	Success(rr, map[string]string{"full_address": "广东省 深圳市 南山区 桃源街道 88号"})

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data := resp.Data.(map[string]interface{})
	addr := data["full_address"].(string)
	// Ensure no unexpected space was inserted between CJK characters
	if addr != "广东省 深圳市 南山区 桃源街道 88号" {
		t.Errorf("full_address = %q, want %q", addr, "广东省 深圳市 南山区 桃源街道 88号")
	}
}

func TestSuccessWithMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	SuccessWithMessage(rr, "created", map[string]string{"id": "123"})

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != CodeSuccess {
		t.Errorf("Code = %d, want %d", resp.Code, CodeSuccess)
	}
	if resp.Message != "created" {
		t.Errorf("Message = %q, want %q", resp.Message, "created")
	}
}
