package service

import (
	"context"
	"testing"
	"time"

	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/internal/model"
)

func TestParserService_Parse(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			URL: "",
		},
		LLM: config.LLMConfig{
			APIKey:  "",
			Model:   "qwen-turbo",
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		},
	}

	svc := NewParserService(cfg, nil)

	tests := []struct {
		name   string
		req    *model.RawFields
		wantOk bool
		method string
	}{
		{
			name: "标准地址-规则引擎成功",
			req: &model.RawFields{
				Name:    "张三",
				Phone:   "15361237638",
				Company: "智腾达软件技术公司",
				Address: "广东省深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室",
			},
			wantOk: true,
			method: MethodRule,
		},
		{
			name: "简称地址-可能需要LLM兜底",
			req: &model.RawFields{
				Address: "南山科技园",
			},
			wantOk: false,
		},
		{
			name: "空地址",
			req: &model.RawFields{
				Address: "",
			},
			wantOk: false,
		},
		{
			name: "不规范地址-缺省省份",
			req: &model.RawFields{
				Address: "深圳市南山区桃源街道88号桑泰大厦13楼",
			},
			wantOk: true,
			method: MethodRule,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := svc.Parse(ctx, tt.req)
			if tt.wantOk {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("expected result, got nil")
				}
				if result.Method != tt.method {
					t.Errorf("expected method %s, got %s", tt.method, result.Method)
				}
				if result.Response.Name != tt.req.Name {
					t.Errorf("expected name %s, got %s", tt.req.Name, result.Response.Name)
				}
				if result.Response.Phone != tt.req.Phone {
					t.Errorf("expected phone %s, got %s", tt.req.Phone, result.Response.Phone)
				}
				if result.ParseTimeMs < 0 {
					t.Errorf("expected non-negative parse time, got %d", result.ParseTimeMs)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			}
		})
	}
}
