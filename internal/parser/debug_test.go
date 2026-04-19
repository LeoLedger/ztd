package parser

import (
	"regexp"
	"testing"
)

func TestDebugRegex(t *testing.T) {
	streetRe := regexp.MustCompile(`^([^\s\d\p{P}]{1,20}?)(街道|镇|乡)`)

	// Test 1: normal case
	s1 := "桃源街道大学城创业园桑泰大厦13楼1303室"
	m1 := streetRe.FindStringSubmatch(s1)
	t.Logf("s1=%q → match=%v", s1, m1)

	// Test 2: what happens with "区桃源街道..." (removing first char)
	s2 := "区桃源街道大学城创业园桑泰大厦13楼1303室"
	m2 := streetRe.FindStringSubmatch(s2)
	t.Logf("s2=%q → match=%v", s2, m2)

	// Test 3: what's actually in the street that causes garbage?
	// Maybe the regex is matching something weird with the non-greedy ?
	s3 := "a桃源街道大学城"
	m3 := streetRe.FindStringSubmatch(s3)
	t.Logf("s3=%q → match=%v", s3, m3)

	// Test 4: the remainder after district extraction in the standard address
	// district="南山区", so remainder = remainder[9:] = remainder[9:]
	// But what's the ORIGINAL remainder before district extraction?
	// Full addr: 广东省深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室
	// After province: 深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室
	// After city: 南山区桃源街道大学城创业园桑泰大厦13楼1303室
	// After district: 桃源街道大学城创业园桑泰大厦13楼1303室
	s4 := "桃源街道大学城创业园桑泰大厦13楼1303室"
	m4 := streetRe.FindStringSubmatch(s4)
	t.Logf("s4=%q → match=%v", s4, m4)
}
