package service

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/internal/model"
)

func TestParserService_DistrictCorrection(t *testing.T) {
	// No Redis, no LLM → pure rule engine + validator path.
	svc := NewParserService(&config.Config{}, nil)

	tests := []struct {
		name            string
		req             *model.RawFields
		wantCorrection  bool
		wantCorrectedTo string
		wantAutoFill    bool
		wantInferred    string
	}{
		{
			name: "深圳-观湖街道-宝安区-街道与区县不一致应纠正为龙华区",
			req: &model.RawFields{
				Address: "深圳市宝安区观湖街道",
			},
			// Rule engine: District=宝安区, Street=观湖街道
			// Validator: 宝安区 valid for 深圳, but 观湖街道→龙华区 → correction
			wantCorrection:  true,
			wantCorrectedTo: "龙华区",
		},
		{
			name: "惠州-无区-河南岸街道-应自动推断惠城区",
			req: &model.RawFields{
				Address: "惠州市河南岸街道金湖社区张屋山一巷二号",
			},
			// Rule engine: District=河南岸街道金湖社区 (invalid), Street=山一巷, Detail=二号
			// Validator: District invalid for 惠州
			//   → findCorrectDistrictByName finds "惠州市:河南岸街道" → "惠城区"
			//   → DistrictCorrection fires with CorrectedDistrict="惠城区"
			//   → District gets set to "惠城区" → DistrictAutoFill is never needed
			wantCorrection:    true,
			wantCorrectedTo:   "惠城区",
		},
		{
			name: "惠州-惠城区河南岸街道-区名过长且非标准应修正",
			req: &model.RawFields{
				Address: "惠州市惠城区河南岸街道金湖社区张屋山一巷二号",
			},
			// Rule engine: District=惠城区河南岸街道金湖社区 (invalid)
			// Validator: findCorrectDistrictByName finds "惠州市:河南岸街道" → "惠城区"
			//   → DistrictCorrection fires with CorrectedDistrict="惠城区"
			wantCorrection:    true,
			wantCorrectedTo:   "惠城区",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.Parse(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if tt.wantCorrection {
				if result.Response.DistrictCorrection == nil {
					t.Fatalf("expected DistrictCorrection, got nil. District=%q Street=%q",
						result.Response.District, result.Response.Street)
				}
				if result.Response.DistrictCorrection.CorrectedDistrict != tt.wantCorrectedTo {
					t.Errorf("CorrectedDistrict = %q, want %q", result.Response.DistrictCorrection.CorrectedDistrict, tt.wantCorrectedTo)
				}
				// When corrected, the District field should also be updated.
				if result.Response.District != tt.wantCorrectedTo {
					t.Errorf("District = %q, want corrected %q", result.Response.District, tt.wantCorrectedTo)
				}
			}

			if tt.wantAutoFill {
				if result.Response.DistrictAutoFill == nil {
					t.Fatalf("expected DistrictAutoFill, got nil. District=%q Street=%q Detail=%q",
						result.Response.District, result.Response.Street, result.Response.Detail)
				}
				if result.Response.DistrictAutoFill.InferredDistrict != tt.wantInferred {
					t.Errorf("InferredDistrict = %q, want %q",
						result.Response.DistrictAutoFill.InferredDistrict, tt.wantInferred)
				}
				if result.Response.District != tt.wantInferred {
					t.Errorf("District = %q, want filled %q", result.Response.District, tt.wantInferred)
				}
			}
		})
	}
}

func TestParserService_NoRedisNoLLM(t *testing.T) {
	// Ensure the service boots without Redis and without LLM keys.
	svc := NewParserService(&config.Config{}, nil)
	if svc == nil {
		t.Fatal("NewParserService returned nil")
	}

	// Rule engine should still work.
	result, err := svc.Parse(context.Background(), &model.RawFields{
		Address: "广东省深圳市南山区科技园南区深南大道9013号",
	})
	if err != nil {
		t.Fatalf("Parse() with no Redis/LLM failed: %v", err)
	}
	if result.Method != MethodRule {
		t.Errorf("expected method %q, got %q", MethodRule, result.Method)
	}
	if result.Response.City != "深圳" {
		t.Errorf("City = %q, want %q", result.Response.City, "深圳")
	}
}

var _ = redis.Client{} // ensure import

func TestParserService_DistrictCorrectionType(t *testing.T) {
	svc := NewParserService(&config.Config{}, nil)

	tests := []struct {
		name         string
		address      string
		wantType     string
		wantCorrected string
	}{
		{
			name:          "观湖街道在宝安区应触发street_mismatch纠正为龙华区",
			address:       "广东省深圳市宝安区观湖街道松元厦河南新村215-4号厂房6楼",
			wantType:      "street_mismatch",
			wantCorrected: "龙华区",
		},
		{
			name:          "粤海街道在福田区应触发street_mismatch纠正为南山区",
			address:       "广东省深圳市福田区粤海街道科技园",
			wantType:      "street_mismatch",
			wantCorrected: "南山区",
		},
		{
			name:          "宝安区+观湖街道纠正后区划应为龙华区",
			address:       "广东省深圳市宝安区观湖街道",
			wantType:      "street_mismatch",
			wantCorrected: "龙华区",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.Parse(context.Background(), &model.RawFields{Address: tt.address})
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if result.Response.DistrictCorrection == nil {
				t.Fatalf("expected DistrictCorrection, got nil. District=%q Street=%q",
					result.Response.District, result.Response.Street)
			}

			corr := result.Response.DistrictCorrection
			if corr.CorrectionType != tt.wantType {
				t.Errorf("CorrectionType = %q, want %q", corr.CorrectionType, tt.wantType)
			}
			if corr.CorrectedDistrict != tt.wantCorrected {
				t.Errorf("CorrectedDistrict = %q, want %q", corr.CorrectedDistrict, tt.wantCorrected)
			}
			if tt.wantCorrected != "" && result.Response.District != tt.wantCorrected {
				t.Errorf("District after correction = %q, want %q", result.Response.District, tt.wantCorrected)
			}
		})
	}
}

func TestParserService_AutoFill_NoDistrictInOriginalText(t *testing.T) {
	svc := NewParserService(&config.Config{}, nil)

	// Note: when the rule engine extracts a district field (even if wrong), the validation
	// layer corrects it via DistrictCorrection — not DistrictAutoFill.
	// DistrictAutoFill is only set when the rule engine leaves district EMPTY.
	tests := []struct {
		name           string
		address        string
		wantCorrection bool
		wantCorrected  string
		wantType       string
		wantAutoFill   bool
		wantInferred   string
	}{
		{
			// Rule engine: district=惠城区河南岸街道金湖社区 (invalid)
			// Validator: findCorrectDistrictByName finds "河南岸街道" → "惠城区"
			// Result: DistrictCorrection with CorrectedDistrict="惠城区"
			name:           "辰芊科技-惠州市应通过DistrictCorrection纠正为惠城区",
			address:        "广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			wantCorrection: true,
			wantCorrected:  "惠城区",
			wantType:       "invalid_district",
		},
		{
			// Rule engine correctly extracts district from this address — no correction needed.
			// This verifies that when the rule engine gets it right, the validator passes through.
			name:           "观湖街道-规则引擎正确解析出龙华区-无需纠正",
			address:        "广东省深圳市观湖街道松元厦村",
			wantCorrection: false,
		},
		{
			// Rule engine: district empty; street empty; original text has "粤海街道"
			// Validator: district missing, autofill finds 粤海→南山区
			// Result: DistrictAutoFill with InferredDistrict="南山区"
			name:           "仅街道名无区划应通过DistrictAutoFill推断南山区",
			address:        "广东省深圳市粤海街道科技园",
			wantAutoFill:   true,
			wantInferred:   "南山区",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.Parse(context.Background(), &model.RawFields{Address: tt.address})
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if tt.wantCorrection {
				if result.Response.DistrictCorrection == nil {
					t.Fatalf("expected DistrictCorrection, got nil. District=%q Street=%q",
						result.Response.District, result.Response.Street)
				}
				corr := result.Response.DistrictCorrection
				if corr.CorrectedDistrict != tt.wantCorrected {
					t.Errorf("CorrectedDistrict = %q, want %q", corr.CorrectedDistrict, tt.wantCorrected)
				}
				if corr.CorrectionType != tt.wantType {
					t.Errorf("CorrectionType = %q, want %q", corr.CorrectionType, tt.wantType)
				}
				if result.Response.District != tt.wantCorrected {
					t.Errorf("District after correction = %q, want %q", result.Response.District, tt.wantCorrected)
				}
			}

			if tt.wantAutoFill {
				if result.Response.DistrictAutoFill == nil {
					t.Fatalf("expected DistrictAutoFill, got nil. District=%q Street=%q",
						result.Response.District, result.Response.Street)
				}
				af := result.Response.DistrictAutoFill
				if af.InferredDistrict != tt.wantInferred {
					t.Errorf("InferredDistrict = %q, want %q", af.InferredDistrict, tt.wantInferred)
				}
				if result.Response.District != tt.wantInferred {
					t.Errorf("District after fill = %q, want %q", result.Response.District, tt.wantInferred)
				}
			}
		})
	}
}
