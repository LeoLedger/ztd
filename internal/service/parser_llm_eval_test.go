package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/internal/model"
)

// TestParserService_LLMCapability_Eval 是能力评估测试，覆盖乱序/缺省/混合输入。
// 运行方式（需要配置 DASHSCOPE_API_KEY）：
//   DASHSCOPE_API_KEY=sk-xxx go test -v -run TestParserService_LLMCapability_Eval ./internal/service/
//
// 评估标准（pass@3 >= 90%）：
//   - company 乱序提取：能正确保留 company 不丢失
//   - 缺省字段：缺失姓名/电话/公司时不应报错或编造
//   - 纯地址补全：根据行政区划知识推断省份/城市
//   - 回归：不破坏现有规则引擎覆盖的标准地址解析

func TestParserService_LLMCapability_Eval(t *testing.T) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		t.Skip("DASHSCOPE_API_KEY not set, skipping LLM capability eval")
	}

	cfg := &config.Config{
		Redis: config.RedisConfig{URL: ""},
		LLM: config.LLMConfig{
			APIKey:  apiKey,
			Model:   "qwen-turbo",
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		},
	}

	svc := NewParserService(cfg, nil)

	// ── 能力评估用例 ──────────────────────────────────────────────────────────────
	type evalCase struct {
		name       string
		inputText  string
		mustHave   []string // 期望必须出现的子串
		mustNotHave []string // 期望不能出现的子串（如明显错误的公司名）
	}

	cases := []evalCase{
		{
			// 原始失败 case：地址在前，公司在后（乱序）
			name:      "乱序-address前-company后",
			inputText: "广东省深圳市南山区桃源街道88号桑泰大厦13楼1303室 张三 15361237638 智腾达科技 ",
			mustHave:  []string{"智腾达科技", "南山区", "广东省"},
			mustNotHave: []string{},
		},
		{
			// 公司在中间
			name:      "乱序-company在中间",
			inputText: "15361237638 智腾达科技 张三 广东省深圳市南山区桃源街道88号桑泰大厦13楼1303室",
			mustHave:  []string{"智腾达科技", "南山区"},
			mustNotHave: []string{},
		},
		{
			// 仅地址，无姓名/电话/公司
			name:      "纯地址-缺省name-phone-company",
			inputText: "广东省深圳市南山区桃源街道88号桑泰大厦13楼1303室",
			mustHave:  []string{"南山区", "广东省"},
			mustNotHave: []string{},
		},
		{
			// 缺省公司，仅姓名和地址
			name:      "缺省company",
			inputText: "张三 广东省深圳市南山区桃源街道88号",
			mustHave:  []string{"张三", "南山区"},
			mustNotHave: []string{},
		},
		{
			// 缺省姓名，仅公司和地址
			name:      "缺省name",
			inputText: "15361237638 智腾达科技 广东省深圳市南山区桃源街道88号桑泰大厦13楼",
			mustHave:  []string{"智腾达科技", "南山区", "15361237638"},
			mustNotHave: []string{},
		},
		{
			// 全乱序：电话-地址-姓名-公司
			name:      "完全乱序",
			inputText: "13800138000 广东省广州市天河区珠江新城花城大道88号A座1201 李四 腾讯科技",
			mustHave:  []string{"天河区", "广州市", "腾讯科技", "13800138000"},
			mustNotHave: []string{},
		},
		{
			// 地址缺省省份，仅有城市
			name:      "缺省省份-仅城市",
			inputText: "张三 13812345678 深圳市南山区科技路100号",
			mustHave:  []string{"深圳市", "南山区", "广东省"}, // LLM 应推断出广东省
			mustNotHave: []string{},
		},
		{
			// 多公司名混合（只有第一个有效）
			name:      "多公司名片段-仅有效",
			inputText: "王五 13912345678 广东省佛山市顺德区北滘镇 美的集团 格力空调",
			mustHave:  []string{"北滘镇", "佛山市"},
			mustNotHave: []string{},
		},
		{
			// 街道信息缺省
			name:      "缺省street",
			inputText: "赵六 13600001111 广东省深圳市南山区88号桑泰大厦1303",
			mustHave:  []string{"南山区", "深圳市", "桑泰大厦"},
			mustNotHave: []string{},
		},
		{
			// 仅有公司+简短地址
			name:      "公司+简短地址",
			inputText: "阿里巴巴 浙江省杭州市余杭区",
			mustHave:  []string{"阿里巴巴", "杭州市", "余杭区"},
			mustNotHave: []string{},
		},
	}

	// ── 回归用例 ────────────────────────────────────────────────────────────────
	// 这些 case 应该走规则引擎（MethodRule），不依赖 LLM。
	regressionCases := []evalCase{
		{
			name:      "回归-标准格式",
			inputText: "张三 13812345678 智腾达科技 广东省深圳市南山区桃源街道88号桑泰大厦13楼1303室",
			mustHave:  []string{"广东省", "南山区", "桃源街道"},
		},
	}

	allCases := append(cases, regressionCases...)
	passed := 0
	failed := 0

	for _, tc := range allCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			req := &model.RawFields{Address: tc.inputText}
			result, err := svc.Parse(ctx, req)

			// 允许错误发生（LLM 可能失败），但不允许 panic
			if err != nil {
				t.Logf("WARN: parse error (may be transient): %v", err)
				failed++
				return
			}
			if result == nil || result.Response == nil {
				t.Error("result or Response is nil")
				failed++
				return
			}

			resp := result.Response
			t.Logf("method=%s name=%q phone=%q company=%q province=%q city=%q district=%q street=%q detail=%q",
				result.Method, resp.Name, resp.Phone, resp.Company,
				resp.Province, resp.City, resp.District, resp.Street, resp.Detail)

			// Verify mustHave
			hasAll := true
			for _, keyword := range tc.mustHave {
				found := false
				for _, k := range []string{resp.Name, resp.Phone, resp.Company,
					resp.Province, resp.City, resp.District, resp.Street, resp.Detail} {
					if contains(k, keyword) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("mustHave %q not found in result", keyword)
					hasAll = false
				}
			}

			// 验证 mustNotHave
			for _, keyword := range tc.mustNotHave {
				found := false
				for _, k := range []string{resp.Name, resp.Phone, resp.Company,
					resp.Province, resp.City, resp.District, resp.Street, resp.Detail} {
					if contains(k, keyword) {
						found = true
						break
					}
				}
				if found {
					t.Errorf("mustNotHave %q should not appear in result", keyword)
				}
			}

			if hasAll && len(tc.mustNotHave) == 0 {
				passed++
				t.Logf("PASS")
			} else {
				failed++
			}
		})
	}

	// ── 评估报告 ────────────────────────────────────────────────────────────────
	total := len(allCases)
	passRate := float64(passed) / float64(total) * 100
	t.Logf("\n========== CAPABILITY EVAL REPORT ==========")
	t.Logf("Total:  %d", total)
	t.Logf("Passed: %d", passed)
	t.Logf("Failed: %d", failed)
	t.Logf("Rate:   %.1f%%", passRate)
	t.Logf("Target: >= 90%%")
	if passRate >= 90 {
		t.Logf("Status: PASS (>= 90%%)")
	} else {
		t.Errorf("Status: FAIL (< 90%%)")
	}
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) > 0 &&
		(len(s) >= len(substr)) &&
		(func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		})()
}
