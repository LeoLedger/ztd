//go:build ignore

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func SignRequest(body, timestamp, appSecret string) string {
	h := hmac.New(sha256.New, []byte(appSecret))
	h.Write([]byte(timestamp + body))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func main() {
	// Your exact body as shown in the request
	body := `{"name":"张三","phone":"15361237638","company":"智腾达","address":"广东省深 圳市南山区桃源街道88号"}`
	timestamp := "9999999999" // placeholder - replace with your actual timestamp

	// Test with both known secrets
	for _, secret := range []string{"secret1", "secret2"} {
		sig := SignRequest(body, timestamp, secret)
		fmt.Printf("secret=%q → sig=%q\n", secret, sig)
	}
}
