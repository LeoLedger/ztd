package parser

import (
	"context"
	"testing"

	"github.com/your-name/address-parse/internal/model"
)

func TestDistrictValidator_ValidateAndCorrect_CorrectDistrict(t *testing.T) {
	v := NewDistrictValidator()

	tests := []struct {
		name     string
		city     string
		district string
		street   string
		detail   string
		wantNil  bool // true = expect nil (no correction needed)
	}{
		{"南山区在深圳市有效", "深圳市", "南山区", "", "", true},
		{"龙华区在深圳市有效", "深圳市", "龙华区", "", "", true},
		{"天河区在广州市有效", "广州市", "天河区", "", "", true},
		{"江宁区在南京市有效", "南京市", "江宁区", "", "", true},
		{"惠城区在惠州市有效", "惠州市", "惠城区", "", "", true},
		{"带新区后缀也是有效的", "深圳市", "大鹏新区", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.ValidateAndCorrect(tt.city, tt.district, tt.street, tt.detail)
			if !tt.wantNil && got == nil {
				t.Errorf("expected non-nil correction, got nil")
			}
			if tt.wantNil && got != nil {
				t.Errorf("expected nil, got correction: %+v", got)
			}
		})
	}
}

func TestDistrictValidator_ValidateAndCorrect_CorrectsDistrict(t *testing.T) {
	v := NewDistrictValidator()

	tests := []struct {
		name               string
		city               string
		district           string
		street             string
		detail             string
		wantCorrection     bool
		wantCorrectedTo    string
		wantReasonContains string
	}{
		{
			name:               "宝安区不在深圳市-观湖街道应纠正为龙华区",
			city:               "深圳市",
			district:           "宝安区",
			street:             "观湖街道",
			detail:             "深圳智腾达软件技术有限公司",
			wantCorrection:     true,
			wantCorrectedTo:    "龙华区",
			wantReasonContains: "区县",
		},
		{
			name:               "南山区不在惠州市",
			city:               "惠州市",
			district:           "南山区",
			street:             "",
			detail:             "",
			wantCorrection:     true,
			wantCorrectedTo:    "",
			wantReasonContains: "区划不在",
		},
		{
			name:               "无法纠正时reason包含区划不在该城市范围内",
			city:               "广州市",
			district:           "南山区",
			street:             "",
			detail:             "",
			wantCorrection:     true,
			wantCorrectedTo:    "",
			wantReasonContains: "区划不在",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.ValidateAndCorrect(tt.city, tt.district, tt.street, tt.detail)
			if tt.wantCorrection && got == nil {
				t.Fatalf("expected correction, got nil")
			}
			if !tt.wantCorrection && got != nil {
				t.Fatalf("expected no correction, got: %+v", got)
			}
			if tt.wantCorrection {
				if got.InputDistrict != tt.district {
					t.Errorf("InputDistrict = %q, want %q", got.InputDistrict, tt.district)
				}
				if got.CorrectedDistrict != tt.wantCorrectedTo {
					t.Errorf("CorrectedDistrict = %q, want %q", got.CorrectedDistrict, tt.wantCorrectedTo)
				}
				if tt.wantReasonContains != "" && !containsString(got.Reason, tt.wantReasonContains) {
					t.Errorf("Reason = %q, want to contain %q", got.Reason, tt.wantReasonContains)
				}
			}
		})
	}
}

func TestDistrictValidator_AutoFill(t *testing.T) {
	v := NewDistrictValidator()

	tests := []struct {
		name              string
		city              string
		district          string
		street            string
		detail            string
		wantAutoFill      bool
		wantInferred      string
		wantSource        string
	}{
		{
			name:           "惠州市河南岸街道应自动推断为惠城区",
			city:           "惠州市",
			district:       "",
			street:         "河南岸街道金湖社区",
			detail:         "张屋山一巷二号",
			wantAutoFill:   true,
			wantInferred:   "惠城区",
			wantSource:     "street_name",
		},
		{
			name:           "惠州市无街道信息时不自动填充",
			city:           "惠州市",
			district:       "",
			street:         "",
			detail:         "张屋山一巷二号",
			wantAutoFill:   false,
			wantInferred:   "",
			wantSource:     "",
		},
		{
			name:           "district已有值时不填充",
			city:           "深圳市",
			district:       "南山区",
			street:         "粤海街道",
			detail:         "",
			wantAutoFill:   false,
			wantInferred:   "",
			wantSource:     "",
		},
		{
			name:           "惠州市河南岸街道自动推断-精确匹配",
			city:           "惠州市",
			district:       "",
			street:         "河南岸街道",
			detail:         "",
			wantAutoFill:   true,
			wantInferred:   "惠城区",
			wantSource:     "street_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.AutoFillDistrict(context.Background(), tt.city, tt.district, tt.street, tt.detail)
			if tt.wantAutoFill && got == nil {
				t.Fatalf("expected auto-fill, got nil")
			}
			if !tt.wantAutoFill && got != nil {
				t.Fatalf("expected no auto-fill, got: %+v", got)
			}
			if tt.wantAutoFill && got != nil {
				if got.InferredDistrict != tt.wantInferred {
					t.Errorf("InferredDistrict = %q, want %q", got.InferredDistrict, tt.wantInferred)
				}
				if got.InferenceSource != tt.wantSource {
					t.Errorf("InferenceSource = %q, want %q", got.InferenceSource, tt.wantSource)
				}
			}
		})
	}
}

func TestDistrictValidator_StreetToDistrictMappings(t *testing.T) {
	v := NewDistrictValidator()

	// Test that key street mappings work correctly.
	tests := []struct {
		city    string
		street  string
		want    string
	}{
		{"深圳市", "观湖街道", "龙华区"},
		{"深圳市", "粤海街道", "南山区"},
		{"深圳市", "华强北街道", "福田区"},
		{"惠州市", "河南岸街道", "惠城区"},
		{"惠州市", "淡水街道", "惠阳区"},
		{"杭州市", "未来科技城", "余杭区"},
		{"上海市", "陆家嘴", "浦东新区"},
		{"上海市", "张江", "浦东新区"},
		{"北京市", "中关村", "海淀区"},
		{"武汉市", "光谷", "洪山区"},
	}

	for _, tt := range tests {
		t.Run(tt.city+"_"+tt.street, func(t *testing.T) {
			got := v.inferFromStreet(tt.city, tt.street)
			if got != tt.want {
				t.Errorf("inferFromStreet(%q, %q) = %q, want %q", tt.city, tt.street, got, tt.want)
			}
		})
	}
}

func TestDistrictValidator_InferFromDetail(t *testing.T) {
	v := NewDistrictValidator()

	tests := []struct {
		city   string
		detail string
		want   string
	}{
		{"惠州市", "辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号", "惠城区"},
		{"深圳市", "科技园南区深南大道9013号", "南山区"},
	}

	for _, tt := range tests {
		t.Run(tt.city+"_"+tt.detail, func(t *testing.T) {
			got := v.inferFromDetail(tt.city, tt.detail)
			if got != tt.want {
				t.Errorf("inferFromDetail(%q, %q) = %q, want %q", tt.city, tt.detail, got, tt.want)
			}
		})
	}
}

func TestDistrictValidator_InferFromOriginalText(t *testing.T) {
	v := NewDistrictValidator()

	tests := []struct {
		name        string
		city        string
		originalText string
		want        string
	}{
		{
			name:        "原始文本含河南岸街道应推断惠城区",
			city:        "惠州市",
			originalText: "广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			want:        "惠城区",
		},
		{
			name:        "原始文本含粤海街道应推断南山区",
			city:        "深圳市",
			originalText: "深圳市粤海街道科技园南区深南大道9013号",
			want:        "南山区",
		},
		{
			name:        "detail为空但originalText含街道名",
			city:        "惠州市",
			originalText: "辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			want:        "惠城区",
		},
		{
			name:        "无匹配街道返回空",
			city:        "惠州市",
			originalText: "张屋山一巷二号",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.inferFromOriginalText(tt.city, tt.originalText)
			if got != tt.want {
				t.Errorf("inferFromOriginalText(%q, %q) = %q, want %q", tt.city, tt.originalText, got, tt.want)
			}
		})
	}
}

func TestDistrictValidator_AutoFillDistrictWithOriginal(t *testing.T) {
	v := NewDistrictValidator()

	tests := []struct {
		name         string
		city         string
		district     string
		street       string
		detail       string
		originalText string
		wantAutoFill bool
		wantInferred string
		wantSource   string
	}{
		{
			name:         "detail为空但originalText含河南岸街道",
			city:         "惠州市",
			district:     "",
			street:       "",
			detail:       "",
			originalText: "广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			wantAutoFill: true,
			wantInferred: "惠城区",
			wantSource:   "original_text",
		},
		{
			name:         "detail内容不足originalText含粤海街道",
			city:         "深圳市",
			district:     "",
			street:       "",
			detail:       "张屋山一巷二号",
			originalText: "广东省深圳市粤海街道科技园张屋山一巷二号",
			wantAutoFill: true,
			wantInferred: "南山区",
			wantSource:   "original_text",
		},
		{
			name:         "originalText无匹配返回nil",
			city:         "惠州市",
			district:     "",
			street:       "",
			detail:       "张屋山一巷二号",
			originalText: "张屋山一巷二号",
			wantAutoFill: false,
			wantInferred: "",
			wantSource:   "",
		},
		{
			name:         "已有有效区划不填充",
			city:         "深圳市",
			district:     "南山区",
			street:       "",
			detail:       "",
			originalText: "深圳市南山区科技园",
			wantAutoFill: false,
			wantInferred: "",
			wantSource:   "",
		},
		{
			name:         "street_name优先于original_text",
			city:         "惠州市",
			district:     "",
			street:       "河南岸街道",
			detail:       "",
			originalText: "广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			wantAutoFill: true,
			wantInferred: "惠城区",
			wantSource:   "street_name",
		},
		{
			name:         "detail_address优先于original_text",
			city:         "惠州市",
			district:     "",
			street:       "",
			detail:       "辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			originalText: "广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			wantAutoFill: true,
			wantInferred: "惠城区",
			wantSource:   "detail_address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.AutoFillDistrictWithOriginal(context.Background(), tt.city, tt.district, tt.street, tt.detail, tt.originalText)
			if tt.wantAutoFill && got == nil {
				t.Fatalf("expected auto-fill, got nil")
			}
			if !tt.wantAutoFill && got != nil {
				t.Fatalf("expected no auto-fill, got: %+v", got)
			}
			if tt.wantAutoFill && got != nil {
				if got.InferredDistrict != tt.wantInferred {
					t.Errorf("InferredDistrict = %q, want %q", got.InferredDistrict, tt.wantInferred)
				}
				if got.InferenceSource != tt.wantSource {
					t.Errorf("InferenceSource = %q, want %q", got.InferenceSource, tt.wantSource)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNormalizeDistrict(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"南山区", "南山区"},
		{"龙华区", "龙华区"},
		{"惠城区", "惠城区"},
		{"宝安区", "宝安区"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeDistrict(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDistrict(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeStreet(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"观湖街道", "观湖"},
		{"粤海街道", "粤海"},
		{"河南岸街道金湖社区", "河南岸街道金湖"}, // strips trailing 社区
		{"金湖社区", "金湖"},              // strips trailing 社区
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeStreet(tt.input)
			if got != tt.want {
				t.Errorf("normalizeStreet(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

var _ = model.DistrictCorrection{} // ensure model import is used

func TestDistrictValidator_CorrectionType(t *testing.T) {
	v := NewDistrictValidator()

	tests := []struct {
		name          string
		city          string
		district     string
		street       string
		detail       string
		wantNil       bool
		wantType      string
		wantCorrected string
	}{
		// street_mismatch: district valid for city but street belongs to different district
		{
			name:          "宝安区+观湖街道应触发street_mismatch纠正为龙华区",
			city:          "深圳市",
			district:      "宝安区",
			street:        "观湖街道",
			detail:        "松元厦河南新村215-4号厂房6楼",
			wantNil:       false,
			wantType:      "street_mismatch",
			wantCorrected: "龙华区",
		},
		{
			name:          "福田区+粤海街道应触发street_mismatch纠正为南山区",
			city:          "深圳市",
			district:      "福田区",
			street:        "粤海街道",
			detail:        "",
			wantNil:       false,
			wantType:      "street_mismatch",
			wantCorrected: "南山区",
		},
		{
			name:          "宝安区+坂田街道应触发street_mismatch纠正为龙岗区",
			city:          "深圳市",
			district:      "宝安区",
			street:        "坂田街道",
			detail:        "",
			wantNil:       false,
			wantType:      "street_mismatch",
			wantCorrected: "龙岗区",
		},
		// invalid_district: district not valid for city at all
		{
			name:          "广州市下惠城区应触发invalid_district",
			city:          "广州市",
			district:      "惠城区",
			street:        "",
			detail:        "",
			wantNil:       false,
			wantType:      "invalid_district",
			wantCorrected: "",
		},
		{
			name:          "惠州市下南山区应触发invalid_district",
			city:          "惠州市",
			district:      "南山区",
			street:        "",
			detail:        "",
			wantNil:       false,
			wantType:      "invalid_district",
			wantCorrected: "",
		},
		// No correction needed
		{
			name:     "南山区在深圳市无需纠正",
			city:     "深圳市",
			district: "南山区",
			street:   "粤海街道",
			detail:   "",
			wantNil:  true,
		},
		// Street name used as district value (the core bug scenario):
		// "河南岸街道" strips to "河南岸" which matches cityDistricts key suffix "惠城" in "惠城区"
		// → detectStreetNameMasqueradingAsDistrict must catch this first
		{
			name:          "河南岸街道在惠州市应被识别为街道名并纠正为惠城区",
			city:          "惠州市",
			district:      "河南岸街道",
			street:        "",
			detail:        "",
			wantNil:       false,
			wantType:      "invalid_district",
			wantCorrected: "惠城区",
		},
		{
			name:          "淡水街道在惠州市应被识别为街道名并纠正为惠阳区",
			city:          "惠州市",
			district:      "淡水街道",
			street:        "",
			detail:        "",
			wantNil:       false,
			wantType:      "invalid_district",
			wantCorrected: "惠阳区",
		},
		{
			name:          "粤海街道在深圳市应被识别为街道名并纠正为南山区",
			city:          "深圳市",
			district:      "粤海街道",
			street:        "",
			detail:        "",
			wantNil:       false,
			wantType:      "invalid_district",
			wantCorrected: "南山区",
		},
		{
			name:          "观湖街道在深圳市应被识别为街道名并纠正为龙华区",
			city:          "深圳市",
			district:      "观湖街道",
			street:        "",
			detail:        "",
			wantNil:       false,
			wantType:      "invalid_district",
			wantCorrected: "龙华区",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.ValidateAndCorrect(tt.city, tt.district, tt.street, tt.detail)
			if tt.wantNil && got != nil {
				t.Fatalf("expected nil, got: %+v", got)
			}
			if !tt.wantNil && got == nil {
				t.Fatalf("expected non-nil correction, got nil")
			}
			if !tt.wantNil {
				if got.CorrectionType != tt.wantType {
					t.Errorf("CorrectionType = %q, want %q", got.CorrectionType, tt.wantType)
				}
				if got.CorrectedDistrict != tt.wantCorrected {
					t.Errorf("CorrectedDistrict = %q, want %q", got.CorrectedDistrict, tt.wantCorrected)
				}
			}
		})
	}
}

func TestDistrictValidator_NewStreetEntries(t *testing.T) {
	v := NewDistrictValidator()

	// Correction tests: use ValidateAndCorrect
	corrTests := []struct {
		name           string
		city           string
		district       string
		street         string
		detail         string
		wantCorrection bool
		wantCorrected  string
		wantType       string
	}{
		{
			name:           "宝安区+盐田港应触发street_mismatch纠正为盐田区",
			city:           "深圳市",
			district:       "宝安区",
			street:         "盐田港",
			detail:         "",
			wantCorrection: true,
			wantCorrected:  "盐田区",
			wantType:       "street_mismatch",
		},
		{
			name:           "宝安区+海山街道应触发street_mismatch纠正为盐田区",
			city:           "深圳市",
			district:       "宝安区",
			street:         "海山街道",
			detail:         "",
			wantCorrection: true,
			wantCorrected:  "盐田区",
			wantType:       "street_mismatch",
		},
		{
			name:           "盐田区+沙头角街道无需纠正",
			city:           "深圳市",
			district:       "盐田区",
			street:         "沙头角街道",
			detail:         "",
			wantCorrection: false,
		},
		{
			name:           "宝安区+鹭湖应触发street_mismatch纠正为龙华区",
			city:           "深圳市",
			district:       "宝安区",
			street:         "鹭湖",
			detail:         "",
			wantCorrection: true,
			wantCorrected:  "龙华区",
			wantType:       "street_mismatch",
		},
		{
			name:           "宝安区+松元厦应触发street_mismatch纠正为龙华区",
			city:           "深圳市",
			district:       "宝安区",
			street:         "松元厦",
			detail:         "",
			wantCorrection: true,
			wantCorrected:  "龙华区",
			wantType:       "street_mismatch",
		},
	}

	for _, tt := range corrTests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.ValidateAndCorrect(tt.city, tt.district, tt.street, tt.detail)
			if tt.wantCorrection && got == nil {
				t.Fatalf("expected correction, got nil")
			}
			if !tt.wantCorrection && got != nil {
				t.Fatalf("expected no correction, got: %+v", got)
			}
			if tt.wantCorrection && got != nil {
				if got.CorrectedDistrict != tt.wantCorrected {
					t.Errorf("CorrectedDistrict = %q, want %q", got.CorrectedDistrict, tt.wantCorrected)
				}
				if got.CorrectionType != tt.wantType {
					t.Errorf("CorrectionType = %q, want %q", got.CorrectionType, tt.wantType)
				}
			}
		})
	}

	// Autofill tests: use AutoFillDistrictWithOriginal
	autoFillTests := []struct {
		name         string
		city         string
		district     string
		street       string
		detail       string
		originalText string
		wantAutoFill bool
		wantInferred string
		wantSource   string
	}{
		{
			name:         "盐田区缺失-通过AutoFillDistrictWithOriginal应自动填充盐田区",
			city:         "深圳市",
			district:     "",
			street:       "",
			detail:       "",
			originalText: "广东省深圳市海山街道盐田港保税区",
			wantAutoFill: true,
			wantInferred: "盐田区",
			wantSource:   "original_text",
		},
	}

	for _, tt := range autoFillTests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.AutoFillDistrictWithOriginal(context.Background(), tt.city, tt.district, tt.street, tt.detail, tt.originalText)
			if tt.wantAutoFill && got == nil {
				t.Fatalf("expected auto-fill, got nil")
			}
			if !tt.wantAutoFill && got != nil {
				t.Fatalf("expected no auto-fill, got: %+v", got)
			}
			if tt.wantAutoFill && got != nil {
				if got.InferredDistrict != tt.wantInferred {
					t.Errorf("InferredDistrict = %q, want %q", got.InferredDistrict, tt.wantInferred)
				}
				if got.InferenceSource != tt.wantSource {
					t.Errorf("InferenceSource = %q, want %q", got.InferenceSource, tt.wantSource)
				}
			}
		})
	}
}

func TestDistrictValidator_AutoFill_NewScenarios(t *testing.T) {
	v := NewDistrictValidator()

	tests := []struct {
		name         string
		city         string
		district     string
		street       string
		detail       string
		originalText string
		wantAutoFill bool
		wantInferred string
		wantSource   string
	}{
		// First user scenario: no district, has 河南岸街道 in original text
		{
			name:         "用户场景1-辰芊科技无区划应自动填充惠城区",
			city:         "惠州市",
			district:     "",
			street:       "",
			detail:       "",
			originalText: "广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			wantAutoFill: true,
			wantInferred: "惠城区",
			wantSource:   "original_text",
		},
		// First user scenario: street extracted, should fill via street
		{
			name:         "用户场景1-提取到河南岸街道应通过street_name填充惠城区",
			city:         "惠州市",
			district:     "",
			street:       "河南岸街道",
			detail:       "金湖社区张屋山一巷二号",
			originalText: "广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			wantAutoFill: true,
			wantInferred: "惠城区",
			wantSource:   "street_name",
		},
		// Second user scenario: district present but wrong (宝安区 vs 龙华区).
		// AutoFillDistrictWithOriginal returns nil when district is valid for city.
		// ValidateAndCorrect is the correct method for this "wrong district" case.
		{
			name:         "宝安区+观湖街道-AutoFill返回nil-用ValidateAndCorrect纠错",
			city:         "深圳市",
			district:     "宝安区",
			street:       "观湖街道",
			detail:       "",
			originalText: "广东省深圳市宝安区观湖街道松元厦河南新村",
			wantAutoFill: false, // 宝安区 is valid for 深圳市, autofill returns nil
			wantInferred: "",
			wantSource:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.AutoFillDistrictWithOriginal(context.Background(), tt.city, tt.district, tt.street, tt.detail, tt.originalText)
			if tt.wantAutoFill && got == nil {
				t.Fatalf("expected auto-fill, got nil")
			}
			if !tt.wantAutoFill && got != nil {
				t.Fatalf("expected no auto-fill, got: %+v", got)
			}
			if tt.wantAutoFill && got != nil {
				if got.InferredDistrict != tt.wantInferred {
					t.Errorf("InferredDistrict = %q, want %q", got.InferredDistrict, tt.wantInferred)
				}
				if got.InferenceSource != tt.wantSource {
					t.Errorf("InferenceSource = %q, want %q", got.InferenceSource, tt.wantSource)
				}
			}
		})
	}
}
