//go:build ignore

package main

import (
	"fmt"
	"regexp"
	"strings"
)

func main() {
	cityRe := regexp.MustCompile(`([^省民自治区特别行政区]+(?:市|地区|区|县))`)
	distRe := regexp.MustCompile(`([^市]+(?:区|县|市辖区))`)

	cases := []string{
		"深圳市南山区桃源街道",
		"浦东新区张江高科技园区碧波路690号",
		"杭州市西湖区文三路398号",
		"呼和浩特市新城区",
		"深圳宝安流塘工业路14号",
		"深圳宝安区流塘工业路3-1",
	}

	for _, s := range cases {
		fmt.Printf("Input: %s\n", s)
		if m := cityRe.FindString(s); m != "" {
			idx := strings.Index(s, m)
			rest := s[idx+len(m):]
			fmt.Printf("  City: %q, rest: %q\n", m, rest)
			if m2 := distRe.FindString(rest); m2 != "" {
				idx2 := strings.Index(rest, m2)
				rest2 := rest[idx2+len(m2):]
				fmt.Printf("  District: %q, rest2: %q\n", m2, rest2)
			}
		}
		fmt.Println()
	}
}
