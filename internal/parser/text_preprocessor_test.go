package parser

import (
	"testing"
)

func TestExtractFields_Standard(t *testing.T) {
	fields := ExtractFields("张三 13812345678 智腾达科技 广东省深圳市南山区桃源街道88号")
	if fields.Name != "张三" {
		t.Errorf("Name = %q, want %q", fields.Name, "张三")
	}
	if fields.Phone != "13812345678" {
		t.Errorf("Phone = %q, want %q", fields.Phone, "13812345678")
	}
	if fields.Company != "智腾达科技" {
		t.Errorf("Company = %q, want %q", fields.Company, "智腾达科技")
	}
	// Address is whatever remains after name/phone/company are removed.
	if fields.Address == "" {
		t.Error("Address is empty, want non-empty")
	}
}

func TestExtractFields_Reordered(t *testing.T) {
	// Phone first, then address, then name — extraction should still work.
	fields := ExtractFields("13812345678 广东省深圳市南山区 张三")
	if fields.Phone != "13812345678" {
		t.Errorf("Phone = %q, want %q", fields.Phone, "13812345678")
	}
	if fields.Name != "张三" {
		t.Errorf("Name = %q, want %q", fields.Name, "张三")
	}
	// Address contains the province/city/district portion.
	if fields.Address == "" {
		t.Error("Address is empty, want non-empty")
	}
}

func TestExtractFields_AddressOnly(t *testing.T) {
	// Only address, no name/phone/company.
	fields := ExtractFields("广东省深圳市南山区桃源街道88号")
	if fields.Name != "" {
		t.Errorf("Name = %q, want empty", fields.Name)
	}
	if fields.Phone != "" {
		t.Errorf("Phone = %q, want empty", fields.Phone)
	}
	if fields.Company != "" {
		t.Errorf("Company = %q, want empty", fields.Company)
	}
	if fields.Address == "" {
		t.Error("Address is empty, want non-empty")
	}
}

func TestExtractFields_PhoneWithSeparators(t *testing.T) {
	fields := ExtractFields("张三 138-1234-5678 广东省深圳市南山区")
	if fields.Phone != "13812345678" {
		t.Errorf("Phone = %q, want %q (separators stripped)", fields.Phone, "13812345678")
	}
}

func TestExtractFields_Landline(t *testing.T) {
	fields := ExtractFields("张三 0755-12345678 广东省深圳市南山区")
	if fields.Phone != "075512345678" {
		t.Errorf("Phone = %q, want %q", fields.Phone, "075512345678")
	}
}

func TestExtractFields_TollFree(t *testing.T) {
	fields := ExtractFields("客服 400-123-4567 广东省深圳市南山区桃源街道88号")
	if fields.Phone != "4001234567" {
		t.Errorf("Phone = %q, want %q", fields.Phone, "4001234567")
	}
}

func TestExtractFields_ContactPrefix(t *testing.T) {
	fields := ExtractFields("收件人张三 13812345678 广东省深圳市南山区")
	if fields.Name != "张三" {
		t.Errorf("Name = %q, want %q", fields.Name, "张三")
	}
}

func TestExtractFields_CompanyWithMarker(t *testing.T) {
	fields := ExtractFields("张三 13812345678 智腾达科技有限公司 广东省深圳市南山区桃源街道88号")
	if fields.Company != "智腾达科技有限公司" {
		t.Errorf("Company = %q, want %q", fields.Company, "智腾达科技有限公司")
	}
}

func TestExtractFields_LoosePhoneFormat(t *testing.T) {
	fields := ExtractFields("张三 联系电话 82345678 广东省深圳市南山区")
	if fields.Phone != "82345678" {
		t.Errorf("Phone = %q, want %q", fields.Phone, "82345678")
	}
}

func TestExtractFields_EmptyString(t *testing.T) {
	fields := ExtractFields("")
	if fields.Name != "" || fields.Phone != "" || fields.Company != "" || fields.Address != "" {
		t.Errorf("all fields should be empty, got Name=%q Phone=%q Company=%q Address=%q",
			fields.Name, fields.Phone, fields.Company, fields.Address)
	}
}

func TestExtractFields_CompanyNoMarker(t *testing.T) {
	// Company without address markers — may not be detected.
	fields := ExtractFields("腾讯科技 13812345678")
	if fields.Phone != "13812345678" {
		t.Errorf("Phone = %q, want %q", fields.Phone, "13812345678")
	}
	// Company may or may not be detected; we only check phone worked.
}

func TestExtractFields_MultipleSpaces(t *testing.T) {
	fields := ExtractFields("张三    13812345678   智腾达科技    广东省深圳市南山区")
	if fields.Phone != "13812345678" {
		t.Errorf("Phone = %q, want %q", fields.Phone, "13812345678")
	}
}
