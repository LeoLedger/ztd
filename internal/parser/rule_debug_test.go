package parser

import (
	"testing"
)

func TestRuleEngine_AddressBeforeCompany(t *testing.T) {
	// After Preprocess: CJK spaces stripped from
	// "广东省深圳市南山区桃源街道88号桑泰大厦13楼1303室 张三 15361237638 智腾达科技 "
	addr := "广东省深圳市南山区桃源街道88号桑泰大厦13楼1303室张三15361237638智腾达科技"
	engine := NewRuleEngine()
	r, ok := engine.Parse(addr)
	t.Logf("ok=%v province=%q city=%q district=%q street=%q detail=%q",
		ok, r.Province, r.City, r.District, r.Street, r.Detail)
	if !ok {
		t.Error("RuleEngine should succeed on full address with province/city/district")
	}
	if r.Province != "广东省" {
		t.Errorf("Province=%q, want 广东省", r.Province)
	}
}
