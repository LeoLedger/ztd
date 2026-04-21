package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/internal/model"
)

func TestParserService_ConcurrentParse(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{URL: ""},
		LLM: config.LLMConfig{
			APIKey:  "",
			Model:   "qwen-turbo",
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		},
	}

	svc := NewParserService(cfg, nil)

	addresses := []string{
		"广东省深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室",
		"北京市朝阳区建国路88号SOHO现代城A座1001",
		"上海市浦东新区张江高科技园区碧波路690号",
		"深圳南山科技园深南大道10000号",
		"广州天河区珠江新城花城大道88号A座1201",
		"浙江省杭州市西湖区文三路398号",
		"广东省佛山市顺德区北滘镇",
		"四川省成都市郫都区团结镇",
		"深圳宝安区西乡街道",
		"重庆市渝北区星光大道100号",
	}

	var (
		wg       sync.WaitGroup
		fails    atomic.Int64
		methods  atomic.Int64
		signatureErrors atomic.Int64
		panics   atomic.Int64
	)

	concurrency := 50
	iterations := 20

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				addr := addresses[(idx+j)%len(addresses)]
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				req := &model.RawFields{
					Name:    "测试用户",
					Phone:   "13800138000",
					Company: "测试公司",
					Address: addr,
				}

				func() {
					defer func() {
						if r := recover(); r != nil {
							panics.Add(1)
						}
					}()
					result, err := svc.Parse(ctx, req)
					if err != nil {
						// LLM not configured → empty address may error; non-empty is unexpected
						if addr != "" {
							fails.Add(1)
						}
						return
					}
					if result == nil {
						fails.Add(1)
						return
					}
					if result.Response == nil {
						fails.Add(1)
						return
					}
					if result.Method != MethodRule && result.Method != MethodCache {
						signatureErrors.Add(1)
					}
					methods.Add(1)
				}()
				cancel()
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("concurrent test timed out after 60 seconds")
	}

	total := int64(concurrency * iterations)
	if fails.Load() > 0 {
		t.Errorf("%d/%d requests failed", fails.Load(), total)
	}
	if panics.Load() > 0 {
		t.Errorf("detected %d panics during concurrent execution", panics.Load())
	}
	if signatureErrors.Load() > 0 {
		t.Errorf("%d requests returned unexpected method (signature errors?)", signatureErrors.Load())
	}
	if methods.Load() == 0 {
		t.Error("no successful parse results recorded")
	}
}

func TestParserService_ConcurrentSameAddress(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{URL: ""},
		LLM:   config.LLMConfig{APIKey: "", Model: "qwen-turbo", BaseURL: ""},
	}

	svc := NewParserService(cfg, nil)

	const (
		concurrency = 50
		iterations  = 10
	)

	addr := "广东省深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		wg    sync.WaitGroup
		ok    atomic.Int64
		fails atomic.Int64
	)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				req := &model.RawFields{
					Name:    "张三",
					Phone:   "15361237638",
					Company: "智腾达软件技术公司",
					Address: addr,
				}
				result, err := svc.Parse(ctx, req)
				if err != nil {
					fails.Add(1)
					return
				}
				if result == nil || result.Response == nil {
					fails.Add(1)
					return
				}
				if result.Response.Province != "广东省" {
					t.Errorf("expected Province=广东省, got %q", result.Response.Province)
					fails.Add(1)
					return
				}
				if result.Response.City != "深圳" {
					t.Errorf("expected City=深圳, got %q", result.Response.City)
					fails.Add(1)
					return
				}
				if result.Response.District != "南山区" {
					t.Errorf("expected District=南山区, got %q", result.Response.District)
					fails.Add(1)
					return
				}
				ok.Add(1)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("concurrent test timed out")
	}

	total := int64(concurrency * iterations)
	if fails.Load() > 0 {
		t.Errorf("%d/%d iterations had errors or wrong results", fails.Load(), total)
	}
	t.Logf("Concurrent same-address: %d/%d OK (%d goroutines × %d iterations)",
		ok.Load(), total, concurrency, iterations)
}
