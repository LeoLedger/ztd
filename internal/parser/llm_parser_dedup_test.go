package parser

import "testing"

func TestDeduplicateAdministrativePrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "广东省惠州市重复",
			input: "广东省惠州市广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
			want:  "广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
		},
		{
			name:  "广东省深圳市重复",
			input: "广东省深圳市广东省深圳市南山区科技园",
			want:  "广东省深圳市南山区科技园",
		},
		{
			name:  "广东省广州市重复",
			input: "广东省广州市广东省广州市天河区猎德街道",
			want:  "广东省广州市天河区猎德街道",
		},
		{
			name:  "无重复不变",
			input: "广东省惠州市辰芊科技有限公司河南岸街道",
			want:  "广东省惠州市辰芊科技有限公司河南岸街道",
		},
		{
			name:  "三次重复保留一次",
			input: "广东省惠州市广东省惠州市广东省惠州市辰芊科技",
			want:  "广东省惠州市辰芊科技",
		},
		{
			name:  "直辖市重复",
			input: "北京市北京市东城区东直门",
			want:  "北京市东城区东直门",
		},
		{
			name:  "短字符串无变化",
			input: "广东省惠州市",
			want:  "广东省惠州市",
		},
		{
			name:  "带空格重复",
			input: "广东省惠州市 广东省惠州市 辰芊科技",
			want:  "广东省惠州市辰芊科技",
		},
		{
			name:  "北京市带空格的重复",
			input: "北京市 北京市 朝阳区三里屯",
			want:  "北京市朝阳区三里屯",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicateAdministrativePrefix(tt.input)
			if got != tt.want {
				t.Errorf("deduplicateAdministrativePrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
