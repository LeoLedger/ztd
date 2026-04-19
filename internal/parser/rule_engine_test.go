package parser

import (
	"testing"
)

func TestRuleEngine_Parse(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		name      string
		address   string
		wantProv  string
		wantCity  string
		wantDist  string
		wantStr   string
		expectOk  bool
	}{
		{
			name:     "深圳标准地址",
			address:  "广东省深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室",
			wantProv: "广东省",
			wantCity: "深圳",
			wantDist: "南山区",
			wantStr:  "桃源街道",
			expectOk: true,
		},
		{
			name:     "北京地址",
			address:  "北京市朝阳区建国路88号SOHO现代城A座1001",
			wantProv: "北京市",
			wantCity: "",
			wantDist: "朝阳区",
			wantStr:  "建国路",
			expectOk: true,
		},
		{
			name:     "上海地址",
			address:  "上海市浦东新区张江高科技园区碧波路690号",
			wantProv: "上海市",
			wantCity: "",
			wantDist: "浦东新区",
			wantStr:  "碧波路",
			expectOk: true,
		},
		{
			name:     "简称-南山科技园",
			address:  "深圳南山科技园",
			wantProv: "",
			wantCity: "深圳",
			wantDist: "",
			wantStr:  "",
			expectOk: true,
		},
		{
			name:     "不完整地址",
			address:  "桑泰大厦13楼",
			wantProv: "",
			wantCity: "",
			expectOk: false,
		},
		{
			name:     "空地址",
			address:  "",
			expectOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := engine.Parse(tt.address)
			if ok != tt.expectOk {
				t.Errorf("Parse(%q) ok = %v, want %v", tt.address, ok, tt.expectOk)
				return
			}
			if ok {
				if result.Province != tt.wantProv {
					t.Errorf("Province = %q, want %q", result.Province, tt.wantProv)
				}
				if result.City != tt.wantCity {
					t.Errorf("City = %q, want %q", result.City, tt.wantCity)
				}
				if result.District != tt.wantDist {
					t.Errorf("District = %q, want %q", result.District, tt.wantDist)
				}
				if result.Street != tt.wantStr {
					t.Errorf("Street = %q, want %q", result.Street, tt.wantStr)
				}
			}
		})
	}
}

func TestRuleEngine_ExtractProvince(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		address string
		want    string
		ok      bool
	}{
		{"广东省深圳市南山区", "广东省", true},
		{"北京市朝阳区", "北京市", true},
		{"浙江省杭州市西湖区", "浙江省", true},
		{"内蒙古自治区呼和浩特市", "内蒙古自治区", true},
		{"abc123", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			remainder := tt.address
			prov, ok := engine.extractProvince(&remainder)
			if ok != tt.ok {
				t.Errorf("extractProvince(%q) ok = %v, want %v", tt.address, ok, tt.ok)
				return
			}
			if ok && prov != tt.want {
				t.Errorf("province = %q, want %q", prov, tt.want)
			}
		})
	}
}

func TestHashAddress(t *testing.T) {
	addr := "广东省深圳市南山区"
	h1 := HashAddress(addr)
	h2 := HashAddress(addr)
	h3 := HashAddress("另一个地址")

	if h1 != h2 {
		t.Error("same address should produce same hash")
	}
	if h1 == h3 {
		t.Error("different addresses should produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("SHA256 hash should be 64 hex chars, got %d", len(h1))
	}
}

func TestSerializeAndDeserialize(t *testing.T) {
	engine := NewRuleEngine()
	result, ok := engine.Parse("广东省深圳市南山区桃源街道")
	if !ok {
		t.Fatal("expected parse to succeed")
	}

	data, err := SerializeResponse(result)
	if err != nil {
		t.Fatalf("SerializeResponse failed: %v", err)
	}

	restored, err := DeserializeResponse(data)
	if err != nil {
		t.Fatalf("DeserializeResponse failed: %v", err)
	}
	if restored.Province != result.Province {
		t.Errorf("Province = %q, want %q", restored.Province, result.Province)
	}
	if restored.City != result.City {
		t.Errorf("City = %q, want %q", restored.City, result.City)
	}
	if restored.District != result.District {
		t.Errorf("District = %q, want %q", restored.District, result.District)
	}
}

func TestBuildFullAddress(t *testing.T) {
	engine := NewRuleEngine()
	result, ok := engine.Parse("广东省深圳市南山区桃源街道大学城创业园")
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if result.FullAddr == "" {
		t.Error("FullAddr should not be empty")
	}
	if result.FullAddr != "广东省 深圳 南山区 桃源街道 大学城创业园" {
		t.Errorf("FullAddr = %q, want %q", result.FullAddr, "广东省 深圳 南山区 桃源街道 大学城创业园")
	}
}

func TestRuleEngine_CityAbbrev(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		name     string
		address  string
		wantProv string
		wantCity string
		wantDist string
		wantStr  string
		ok       bool
	}{
		{
			name:     "省略省后的市",
			address:  "深圳南山区",
			wantProv: "",
			wantCity: "深圳",
			wantDist: "南山区",
			ok:       true,
		},
		{
			name:     "省略省后的杭州",
			address:  "杭州市西湖区",
			wantProv: "",
			wantCity: "杭州",
			wantDist: "西湖区",
			ok:       true,
		},
		{
			name:     "仅市名",
			address:  "深圳市",
			wantProv: "",
			wantCity: "深圳",
			wantDist: "",
			ok:       true,
		},
		{
			name:     "广州省略市",
			address:  "广州天河区",
			wantProv: "",
			wantCity: "广州",
			wantDist: "天河区",
			ok:       true,
		},
		{
			name:     "带镇的地址",
			address:  "深圳宝安区西乡街道",
			wantProv: "",
			wantCity: "深圳",
			wantDist: "宝安区",
			wantStr:  "西乡街道",
			ok:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := engine.Parse(tt.address)
			if ok != tt.ok {
				t.Errorf("Parse(%q) ok = %v, want %v", tt.address, ok, tt.ok)
				return
			}
			if ok {
				if result.Province != tt.wantProv {
					t.Errorf("Province = %q, want %q", result.Province, tt.wantProv)
				}
				if result.City != tt.wantCity {
					t.Errorf("City = %q, want %q", result.City, tt.wantCity)
				}
				if result.District != tt.wantDist {
					t.Errorf("District = %q, want %q", result.District, tt.wantDist)
				}
			}
		})
	}
}

func TestRuleEngine_CountyLevel(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		name     string
		address  string
		wantDist string
		ok       bool
	}{
		{
			name:     "普通县",
			address:  "浙江省温州市永嘉县",
			wantDist: "永嘉县",
			ok:       true,
		},
		{
			name:     "县级市",
			address:  "广东省佛山市顺德区大良街道",
			wantDist: "顺德区",
			ok:       true,
		},
		{
			name:     "省略市辖区",
			address:  "上海市静安区",
			wantDist: "静安区",
			ok:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := engine.Parse(tt.address)
			if ok != tt.ok {
				t.Errorf("Parse(%q) ok = %v, want %v", tt.address, ok, tt.ok)
				return
			}
			if ok && result.District != tt.wantDist {
				t.Errorf("District = %q, want %q", result.District, tt.wantDist)
			}
		})
	}
}

func TestRuleEngine_StreetAndDetail(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		name        string
		address     string
		wantStreet  string
		wantDetail  string
		ok          bool
	}{
		{
			name:       "镇",
			address:    "广东省佛山市顺德区北滘镇",
			wantStreet: "北滘镇",
			wantDetail: "",
			ok:        true,
		},
		{
			name:       "乡",
			address:    "四川省成都市郫都区团结镇",
			wantStreet: "团结镇",
			wantDetail: "",
			ok:        true,
		},
		{
			name:       "带门牌号",
			address:    "深圳南山科技园深南大道10000号",
			wantStreet: "深南大道",
			wantDetail: "10000号",
			ok:        true,
		},
		{
			name:       "带楼栋室号",
			address:    "广州天河区珠江新城花城大道88号A座1201",
			wantStreet: "花城大道",
			wantDetail: "88号A座1201",
			ok:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := engine.Parse(tt.address)
			if ok != tt.ok {
				t.Errorf("Parse(%q) ok = %v, want %v", tt.address, ok, tt.ok)
				return
			}
			if ok {
				if result.Street != tt.wantStreet {
					t.Errorf("Street = %q, want %q", result.Street, tt.wantStreet)
				}
				if result.Detail != tt.wantDetail {
					t.Errorf("Detail = %q, want %q", result.Detail, tt.wantDetail)
				}
			}
		})
	}
}

func TestRuleEngine_AutonomousRegions(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		name     string
		address  string
		wantProv string
		wantCity string
		ok       bool
	}{
		{
			name:     "内蒙古简称",
			address:  "内蒙古呼和浩特市新城区",
			wantProv: "内蒙古自治区",
			wantCity: "呼和浩特",
			ok:       true,
		},
		{
			name:     "广西简称",
			address:  "广西南宁市青秀区",
			wantProv: "广西壮族自治区",
			wantCity: "南宁",
			ok:       true,
		},
		{
			name:     "新疆简称",
			address:  "新疆乌鲁木齐市天山区",
			wantProv: "新疆维吾尔自治区",
			wantCity: "乌鲁木齐",
			ok:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := engine.Parse(tt.address)
			if ok != tt.ok {
				t.Errorf("Parse(%q) ok = %v, want %v", tt.address, ok, tt.ok)
				return
			}
			if ok {
				if result.Province != tt.wantProv {
					t.Errorf("Province = %q, want %q", result.Province, tt.wantProv)
				}
				if result.City != tt.wantCity {
					t.Errorf("City = %q, want %q", result.City, tt.wantCity)
				}
			}
		})
	}
}

func TestRuleEngine_SpecialFormats(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		name     string
		address  string
		wantProv string
		wantCity string
		ok       bool
	}{
		{
			name:     "带空格的地址",
			address:  "广东省 深圳市 南山区",
			wantProv: "广东省",
			wantCity: "深圳",
			ok:       true,
		},
		{
			name:     "全角括号",
			address:  "深圳南山区（桃源街道）大学城",
			wantProv: "",
			wantCity: "深圳",
			ok:       true,
		},
		{
			name:     "英文逗号分隔",
			address:  "广东省深圳市,南山区",
			wantProv: "广东省",
			wantCity: "深圳",
			ok:       true,
		},
		{
			name:     "仅数字开头",
			address:  "1号",
			ok:       false,
		},
		{
			name:     "英文字母",
			address:  "ABC Street",
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := engine.Parse(tt.address)
			if ok != tt.ok {
				t.Errorf("Parse(%q) ok = %v, want %v", tt.address, ok, tt.ok)
				return
			}
			if ok {
				if result.Province != tt.wantProv {
					t.Errorf("Province = %q, want %q", result.Province, tt.wantProv)
				}
				if result.City != tt.wantCity {
					t.Errorf("City = %q, want %q", result.City, tt.wantCity)
				}
			}
		})
	}
}

func TestHashAddress_Uniqueness(t *testing.T) {
	inputs := []string{
		"广东省深圳市南山区",
		"北京市朝阳区",
		"上海市浦东新区",
		"广东省深圳市南山区",
	}

	hashes := make(map[string]bool)
	for _, input := range inputs {
		h := HashAddress(input)
		hashes[h] = true
	}

	uniqueCount := len(hashes)
	expectedUnique := 3
	if uniqueCount != expectedUnique {
		t.Errorf("expected %d unique hashes, got %d", expectedUnique, uniqueCount)
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	engine := NewRuleEngine()
	result, ok := engine.Parse("广东省深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室")
	if !ok {
		t.Fatal("expected parse to succeed")
	}

	data, err := SerializeResponse(result)
	if err != nil {
		t.Fatalf("SerializeResponse failed: %v", err)
	}

	restored, err := DeserializeResponse(data)
	if err != nil {
		t.Fatalf("DeserializeResponse failed: %v", err)
	}

	if restored.Province != result.Province {
		t.Errorf("Province mismatch: got %q, want %q", restored.Province, result.Province)
	}
	if restored.City != result.City {
		t.Errorf("City mismatch: got %q, want %q", restored.City, result.City)
	}
	if restored.District != result.District {
		t.Errorf("District mismatch: got %q, want %q", restored.District, result.District)
	}
	if restored.Street != result.Street {
		t.Errorf("Street mismatch: got %q, want %q", restored.Street, result.Street)
	}
	if restored.Detail != result.Detail {
		t.Errorf("Detail mismatch: got %q, want %q", restored.Detail, result.Detail)
	}
	if restored.FullAddr != result.FullAddr {
		t.Errorf("FullAddr mismatch: got %q, want %q", restored.FullAddr, result.FullAddr)
	}
}

func TestPreprocess_CompanySuffixStripping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "深圳南山区（桃源街道）大学城",
			expected: "深圳南山区桃源街道大学城",
		},
		{
			input:    "深圳(南山)科技园",
			expected: "深圳南山科技园",
		},
		{
			input:    "桑泰大厦13楼1303室",
			expected: "桑泰大厦 13楼1303室",
		},
		{
			input:    "不需要处理有限公司",
			expected: "不需要处理有限公司",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Preprocess(tt.input)
			if result != tt.expected {
				t.Errorf("Preprocess(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPreprocess_ParentheticalStripping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "深圳南山区（桃源街道）大学城",
			expected: "深圳南山区桃源街道大学城",
		},
		{
			input:    "深圳(南山)科技园",
			expected: "深圳南山科技园",
		},
		{
			input:    "广州天河区（珠江新城）花城大道",
			expected: "广州天河区珠江新城花城大道",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Preprocess(tt.input)
			if result != tt.expected {
				t.Errorf("Preprocess(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPreprocess_FloorNumberNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "桑泰大厦13楼1303室",
			expected: "桑泰大厦 13楼1303室",
		},
		{
			input:    "现代城A座1201室",
			expected: "现代城A座1201室",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Preprocess(tt.input)
			if result != tt.expected {
				t.Errorf("Preprocess(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRuleEngine_DistrictLevelCities(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		name     string
		address  string
		wantCity string
		wantDist string
		ok       bool
	}{
		{
			name:     "南山区 — standalone (no province) → extracted as District",
			address:  "南山区桃源街道",
			wantCity: "",
			wantDist: "南山区",
			ok:       true,
		},
		{
			name:     "福田区 — standalone (no province) → extracted as District",
			address:  "福田区深南大道100号",
			wantCity: "",
			wantDist: "福田区",
			ok:       true,
		},
		{
			name:     "天河区 — standalone (no province) → extracted as District",
			address:  "天河区珠江新城花城大道88号",
			wantCity: "",
			wantDist: "天河区",
			ok:       true,
		},
		{
			name:     "南山区 followed by explicit district marker → ambiguous, no district extracted",
			address:  "南山区南山区科技园",
			wantCity: "",
			wantDist: "",
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := engine.Parse(tt.address)
			if ok != tt.ok {
				t.Errorf("Parse(%q) ok = %v, want %v", tt.address, ok, tt.ok)
				return
			}
			if ok {
				if result.City != tt.wantCity {
					t.Errorf("City = %q, want %q", result.City, tt.wantCity)
				}
				if result.District != tt.wantDist {
					t.Errorf("District = %q, want %q", result.District, tt.wantDist)
				}
			}
		})
	}
}

func TestRuleEngine_NoProvinceParkKeyword(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		name       string
		address    string
		wantProv   string
		wantCity   string
		wantDist   string
		wantStreet string
		expectOk   bool
	}{
		{
			name:       "无省份前缀-南山区大学城",
			address:    "深圳南山大学城创业园桑泰大厦13楼1303室",
			wantProv:   "",
			wantCity:   "深圳",
			wantDist:   "南山区",
			wantStreet: "",
			expectOk:   true,
		},
		{
			name:       "无省份前缀-福田区科技园",
			address:    "深圳福田CBD中心科技园深南大道100号",
			wantProv:   "",
			wantCity:   "深圳",
			wantDist:   "福田区",
			wantStreet: "深南大道",
			expectOk:   true,
		},
		{
			name:       "无省份前缀-南山区工业园",
			address:    "深圳南山工业园大新路88号",
			wantProv:   "",
			wantCity:   "深圳",
			wantDist:   "南山区",
			wantStreet: "大新路",
			expectOk:   true,
		},
		{
			name:       "无省份前缀-宝安区创业园",
			address:    "深圳宝安创业园航空路1号",
			wantProv:   "",
			wantCity:   "深圳",
			wantDist:   "宝安区",
			wantStreet: "航空路",
			expectOk:   true,
		},
		{
			name:       "带街道的无省份前缀-南山区粤海街道",
			address:    "深圳南山粤海街道科技园南区",
			wantProv:   "",
			wantCity:   "深圳",
			wantDist:   "",
			wantStreet: "南山粤海街道",
			expectOk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := engine.Parse(tt.address)
			if ok != tt.expectOk {
				t.Errorf("Parse(%q) ok = %v, want %v", tt.address, ok, tt.expectOk)
				return
			}
			if ok {
				if result.Province != tt.wantProv {
					t.Errorf("Province = %q, want %q", result.Province, tt.wantProv)
				}
				if result.City != tt.wantCity {
					t.Errorf("City = %q, want %q", result.City, tt.wantCity)
				}
				if result.District != tt.wantDist {
					t.Errorf("District = %q, want %q", result.District, tt.wantDist)
				}
				if result.Street != tt.wantStreet {
					t.Errorf("Street = %q, want %q", result.Street, tt.wantStreet)
				}
			}
		})
	}
}

func TestNormalizeText_WhitespaceAndNewlines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "单换行符",
			input:    "广东省\n深圳市",
			expected: "广东省 深圳市",
		},
		{
			name:     "多换行符",
			input:    "广东省\n\n深圳市\n\n南山区",
			expected: "广东省 深圳市 南山区",
		},
		{
			name:     "混合换行和空格",
			input:    "广东省 \r\n  深圳市\r\n\t南山区",
			expected: "广东省 深圳市 南山区",
		},
		{
			name:     "制表符",
			input:    "广东省\t深圳市",
			expected: "广东省 深圳市",
		},
		{
			name:     "连续空格",
			input:    "广东省   深圳市    南山区",
			expected: "广东省 深圳市 南山区",
		},
		{
			name:     "首尾空白",
			input:    "  广东省深圳市南山区  ",
			expected: "广东省深圳市南山区",
		},
		{
			name:     "全角转半角",
			input:    "广东省深圳市南山区",
			expected: "广东省深圳市南山区",
		},
		{
			name:     "中文标点替换为空格",
			input:    "广东省，深圳市，南山区",
			expected: "广东省 深圳市 南山区",
		},
		{
			name:     "emoji去除-相邻中文合并",
			input:    "广东省😀深圳市📍南山区",
			expected: "广东省深圳市南山区",
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
		{
			name:     "仅空白字符",
			input:    "   \n\t\r   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeText(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStripCJKSpaces(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "CJK字符之间的空格被移除",
			input:    "广东省深 圳市南山区桃源街道88号",
			expected: "广东省深圳市南山区桃源街道88号",
		},
		{
			name:     "南山区",
			input:    "广州南 山区",
			expected: "广州南山区",
		},
		{
			name:     "街道名中含空格",
			input:    "深圳南山粤海 街道科技园",
			expected: "深圳南山粤海街道科技园",
		},
		{
			name:     "数字和中文之间的空格保留",
			input:    "桃源街道 88号",
			expected: "桃源街道 88号",
		},
		{
			name:     "多个CJK-CJK空格",
			input:    "深 圳 市 南 山 区",
			expected: "深圳市南山区",
		},
		{
			name:     "纯空格字符串",
			input:    "   ",
			expected: "   ",
		},
		{
			name:     "无CJK字符不变",
			input:    "ABC 123 Street",
			expected: "ABC 123 Street",
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripCJKSpaces(tt.input)
			if result != tt.expected {
				t.Errorf("stripCJKSpaces(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain JSON",
			input:    `{"province":"广东省","city":"深圳市"}`,
			expected: `{"province":"广东省","city":"深圳市"}`,
		},
		{
			name:     "markdown code fence with json lang",
			input:    "```json\n{\"province\":\"广东省\",\"city\":\"深圳市\"}\n```",
			expected: `{"province":"广东省","city":"深圳市"}`,
		},
		{
			name:     "markdown code fence without lang",
			input:    "```\n{\"province\":\"广东省\"}\n```",
			expected: `{"province":"广东省"}`,
		},
		{
			name:     "leading text before JSON",
			input:    "Here is the result: {\"province\":\"广东省\"}",
			expected: `{"province":"广东省"}`,
		},
		{
			name:     "trailing text after JSON",
			input:    `{"province":"广东省"} and some explanation`,
			expected: `{"province":"广东省"}`,
		},
		{
			name:     "text wrapped JSON",
			input:    "result: ```json\n{\"city\":\"深圳\"}\n```\nplease use it",
			expected: `{"city":"深圳"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanJSONResponse(tt.input)
			if result != tt.expected {
				t.Errorf("cleanJSONResponse(%q)\n  got:  %q\n  want: %q", tt.input, result, tt.expected)
			}
		})
	}
}
