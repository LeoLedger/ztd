//go:build ignore

package main

import (
	"fmt"
	"regexp"
	"strings"
)

func main() {
	cityPatterns := []string{"深圳", "广州", "杭州", "上海", "北京"}
	cityPatternsRe := regexp.MustCompile(fmt.Sprintf(`^(%s)`, strings.Join(cityPatterns, "|")))
	districtRe := regexp.MustCompile(`^(.+?)(区|县|市辖区|县级市)`)

	cases := []string{
		"深圳市南山区",
		"上海市浦东新区张江高科技园区碧波路690号",
	}

	for _, input := range cases {
		fmt.Printf("\n=== Input: %s ===\n", input)

		// Step 1: cityRe
		if cityMatches := cityPatternsRe.FindStringSubmatch(input); len(cityMatches) > 1 {
			city := cityMatches[1]
			rest := cityPatternsRe.ReplaceAllString(input, "")
			rest2 := strings.TrimPrefix(rest, "市")
			fmt.Printf("cityRe: matched=%q, afterReplace=%q, afterStrip市=%q\n", city, rest, rest2)
		}

		// Step 2: district
		if distMatches := districtRe.FindStringSubmatch(input); len(distMatches) > 1 {
			fmt.Printf("districtRe: matched=%q%q, afterReplace=%q\n",
				distMatches[1], distMatches[2],
				districtRe.ReplaceAllString(input, ""))
		}
	}
}
