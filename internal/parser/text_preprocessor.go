package parser

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/your-name/address-parse/internal/model"
)

// Extraction order (phone → name → company → address):
//
//   - Phone goes first because digit patterns are unambiguous and phone numbers
//     can appear anywhere in the string.
//   - Name follows — scans the full original buffer so names at the end are found
//     even when the string starts with a phone number.
//   - Company follows after name and phone are stripped.
//   - Address is whatever remains after the above are stripped.
//
// Each extractor removes its matched segment from the working buffer so that later
// extractors operate on progressively cleaned text.

// ExtractFields attempts to extract name/phone/company/address from a free-text
// input string. Any subset of fields may be present; missing fields are empty.
func ExtractFields(raw string) model.RawFields {
	if raw == "" {
		return model.RawFields{}
	}

	buf := raw
	result := model.RawFields{}

	// Phone first — digit patterns are unambiguous regardless of position.
	if m := extractPhone(buf); m != "" {
		result.Phone = normalizePhone(m)
		buf = removeSegment(buf, m)
		buf = trimSpaces(buf)
	}

	// Name second — scan the post-phone-removal buffer. The name may appear before
	// or after geographic markers; extractName strips geographic prefixes internally.
	if name := extractName(buf); name != "" {
		result.Name = NormalizeText(name)
		buf = removeSegment(buf, name)
		buf = trimSpaces(buf)
	}

	// Company third — after name is gone.
	if company := extractCompany(buf); company != "" {
		result.Company = NormalizeText(company)
		buf = removeSegment(buf, company)
		buf = trimSpaces(buf)
	}

	// Address is whatever remains.
	result.Address = buf
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Phone extraction
// ─────────────────────────────────────────────────────────────────────────────

var (
	// 11-digit Chinese mobile: exactly 11 digits with optional - or space separators.
	// Requires leading "1" so it doesn't accidentally match other long digit strings.
	mobileRe = regexp.MustCompile(`1[3-9]\d[- ]?\d{4}[- ]?\d{4}`)

	// Landline: 0 + 2-4 digit area code + optional separator + 7-8 digit number.
	landlineRe = regexp.MustCompile(`0\d{2,4}[ \-（）()]{0,2}\d{7,8}`)

	// 400/800 business numbers: matches 400-123-4567, 4001234567, 400 123 4567.
	tollFreeRe = regexp.MustCompile(`(?:400|800)[- ]?\d{3,4}[- ]?\d{3,4}`)

	// 7-8 digit number preceded by "电话" etc. (for when the area code is missing).
	loosePhoneRe = regexp.MustCompile(`(?:电话|联络|联系)[：:\s]*(\d{7,8})`)

	// Collapses any run of whitespace into a single space.
	spaceRe = regexp.MustCompile(`\s+`)
)

// extractPhone returns the first phone number found in buf, or "".
func extractPhone(buf string) string {
	if m := mobileRe.FindString(buf); m != "" {
		return m
	}
	if m := tollFreeRe.FindString(buf); m != "" {
		return m
	}
	if m := landlineRe.FindString(buf); m != "" {
		return m
	}
	if m := loosePhoneRe.FindStringSubmatch(buf); len(m) > 1 {
		return m[1]
	}
	return ""
}

// digitsOnly returns only the digit characters in s.
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizePhone returns a contiguous digit string by stripping non-digit characters.
// This preserves the leading "0" in landline numbers (e.g. "0755-12345678" → "075512345678").
func normalizePhone(s string) string {
	return digitsOnly(s)
}

// trimSpaces collapses whitespace runs and trims both ends.
func trimSpaces(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

// ─────────────────────────────────────────────────────────────────────────────
// Name extraction
// ─────────────────────────────────────────────────────────────────────────────

// Top Chinese surnames — used as anchor for name detection.
var surnames = map[string]bool{
	"王": true, "李": true, "张": true, "刘": true, "陈": true, "杨": true,
	"黄": true, "赵": true, "周": true, "吴": true, "徐": true, "孙": true,
	"胡": true, "朱": true, "高": true, "林": true, "何": true, "郭": true,
	"马": true, "罗": true, "梁": true, "宋": true, "郑": true, "谢": true,
	"韩": true, "唐": true, "冯": true, "于": true, "董": true, "萧": true,
	"程": true, "曹": true, "袁": true, "邓": true, "许": true, "傅": true,
	"沈": true, "曾": true, "彭": true, "吕": true, "苏": true, "卢": true,
	"蒋": true, "蔡": true, "贾": true, "丁": true, "魏": true, "薛": true,
	"叶": true, "阎": true, "余": true, "潘": true, "杜": true, "戴": true,
	"夏": true, "钟": true, "汪": true, "田": true, "任": true, "姜": true,
	"范": true, "方": true, "石": true, "姚": true, "谭": true, "廖": true,
	"邹": true, "熊": true, "金": true, "陆": true, "郝": true, "孔": true,
	"白": true, "崔": true, "康": true, "毛": true, "邱": true, "秦": true,
	"江": true, "史": true, "顾": true, "侯": true, "邵": true, "孟": true,
	"龙": true, "万": true, "段": true, "漕": true, "钱": true, "汤": true,
	"尹": true, "黎": true, "易": true, "常": true, "武": true, "乔": true,
	"贺": true, "赖": true, "龚": true, "文": true,
}

// Common 2-character given names that appear without an obvious surname.
var common2CharNames = map[string]bool{
	"建国": true, "建军": true, "保安": true, "和平": true, "志强": true,
	"小红": true, "小芳": true, "小玲": true, "小燕": true,
	"秀英": true, "桂英": true, "建华": true, "永红": true,
}

// extractName scans the full buffer for a Chinese personal name. Returns "" if none found.
// Names can appear anywhere: at the start ("张三 138...") or at the end ("... 张三").
func extractName(buf string) string {
	buf = strings.TrimSpace(buf)
	if buf == "" {
		return ""
	}

	// Strip "收件人: 张三" style prefixes.
	for _, prefix := range []string{
		"收件人", "收货人", "发件人", "姓名", "名字",
		"联系人", "购买者", "订购人", "订购者",
	} {
		if strings.HasPrefix(buf, prefix) {
			rest := strings.TrimLeft(buf[len(prefix):], "：: \t")
			if rest != "" {
				buf = rest
				break
			}
		}
	}

	// Strip geographic prefix so the scan can find names that appear after
	// province/city markers in the buffer.
	buf = stripGeographicPrefix(buf)

	runes := []rune(buf)

	// Strategy 1: known surname + 1-2 character given name — scan original buffer.
	for i := 0; i < len(runes); i++ {
		s1 := string(runes[i])
		if !surnames[s1] {
			continue
		}
		for givenLen := 1; givenLen <= 2; givenLen++ {
			if i+givenLen >= len(runes) {
				break
			}
			given := runes[i+1 : i+1+givenLen]
			valid := true
			for _, r := range given {
				if !unicode.Is(unicode.Han, r) {
					valid = false
					break
				}
			}
			if valid {
				return string(runes[i : i+1+givenLen])
			}
		}
	}

	// Strategy 2: common 2-char given name without a known surname.
	for i := 0; i+2 <= len(runes); i++ {
		pair := string(runes[i : i+2])
		if !common2CharNames[pair] {
			continue
		}
		if i+2 == len(runes) {
			return pair
		}
		next := runes[i+2]
		if unicode.IsSpace(next) || next == '号' || next == '楼' || next == '室' || next == '栋' {
			return pair
		}
	}

	return ""
}

// stripGeographicPrefix removes province/city/district markers from the start of s.
// Returns the same string if no prefix is found.
func stripGeographicPrefix(s string) string {
	// Full-length autonomous regions and special admin areas first.
	for _, p := range []string{
		"内蒙古自治区", "广西壮族自治区", "新疆维吾尔自治区",
		"西藏自治区", "宁夏回族自治区",
		"香港特别行政区", "澳门特别行政区",
	} {
		if strings.HasPrefix(s, p) {
			rest := s[len(p):]
			rest = strings.TrimLeft(rest, "：: \t")
			if rest != "" {
				return rest
			}
		}
	}
	// Short province/city markers.
	for _, p := range []string{
		"省", "市", "区", "县", "镇", "乡",
	} {
		if strings.HasPrefix(s, p) {
			rest := s[len(p):]
			rest = strings.TrimLeft(rest, "：: \t")
			if rest != "" {
				return rest
			}
		}
	}
	// Two-character province/city abbreviations.
	for _, p := range []string{
		"北京", "上海", "天津", "重庆",
		"广东", "浙江", "江苏", "四川", "湖南", "湖北", "山东", "福建",
		"河南", "河北", "山西", "江西", "安徽", "海南", "贵州", "云南",
		"陕西", "甘肃", "青海", "黑龙江", "吉林", "辽宁",
	} {
		if strings.HasPrefix(s, p) {
			rest := s[len(p):]
			rest = strings.TrimLeft(rest, "：: \t")
			if rest != "" {
				return rest
			}
		}
	}
	return s
}

// ─────────────────────────────────────────────────────────────────────────────
// Company extraction
// ─────────────────────────────────────────────────────────────────────────────

// Keywords that reliably mark the end of a company name and start of address.
var addressMarkers = []string{
	"省", "市", "区", "县", "镇", "乡", "街道", "路", "号",
	"栋", "楼", "室", "单元", "弄", "巷", "大道", "广场",
	"大厦", "花园", "中心", "园", "科技园", "工业园",
	"创业园", "物流", "仓", "厂", "场",
}

// Keywords that reliably mark the end of a company name.
var companyMarkers = []string{
	"公司", "集团", "科技", "企业", "有限公司", "股份有限公司",
	"责任公司", "Co.", "LTD", "Ltd", "Inc.",
	"株式会社", "合同会社",
}

// containsCJK returns true if s contains at least one CJK character.
func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// extractCompany returns the company name found in buf, or "".
func extractCompany(buf string) string {
	buf = strings.TrimSpace(buf)
	if buf == "" {
		return ""
	}

	// Strategy A: text before a known company marker ("公司", "科技" …).
	// Scan the entire buffer so company names in the middle are also caught.
	for _, marker := range companyMarkers {
		idx := strings.Index(buf, marker)
		if idx > 2 {
			candidate := strings.TrimSpace(buf[:idx] + marker)
			if isCompanyName(candidate) {
				return candidate
			}
		}
	}

	// Strategy B: short text before an address marker.
	// Only match short candidates (≤ 15 chars) so province/city fragments mid-buffer
	// are not confused with company names.
	for _, marker := range addressMarkers {
		idx := strings.Index(buf, marker)
		if idx > 2 && idx <= 15 {
			candidate := strings.TrimSpace(buf[:idx])
			if isCompanyName(candidate) {
				return candidate
			}
		}
	}

	return ""
}

// isCompanyName returns true if candidate is a plausible Chinese company name.
// A valid company name must:
//   - be at least 4 characters long, AND
//   - either end with a known company marker keyword ("公司", "科技", etc.)
//     OR NOT end with a province/city abbreviation.
func isCompanyName(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return false
	}
	// Must end with a company marker.
	for _, m := range companyMarkers {
		if strings.HasSuffix(s, m) {
			return true
		}
	}
	// Otherwise reject if it ends with a province/city abbreviation.
	return !isAddressSuffix(s)
}

// isAddressSuffix returns true if s ends with a province or city abbreviation.
var provinceCitySuffixes = []string{
	// Provinces
	"内蒙古", "广西", "新疆", "西藏", "宁夏",
	"广东", "浙江", "江苏", "四川", "湖南", "湖北", "山东", "福建",
	"河南", "河北", "山西", "江西", "安徽", "海南", "贵州", "云南",
	"陕西", "甘肃", "青海", "黑龙江", "吉林", "辽宁",
	// Special admin areas
	"香港", "澳门",
	// Direct-admin cities
	"北京", "上海", "天津", "重庆",
	// Prefecture-level cities that commonly appear mid-address (e.g. "广东省深圳")
	"深圳", "广州", "成都", "杭州", "武汉", "西安", "南京", "苏州",
	"东莞", "佛山", "珠海", "中山", "惠州", "汕头", "湛江", "江门",
	"宁波", "温州", "无锡", "常州", "南通", "徐州", "泉州", "厦门",
	"福州", "济南", "青岛", "烟台", "威海", "大连", "沈阳", "哈尔滨",
	"长春", "南昌", "合肥", "南宁", "贵阳", "昆明", "拉萨", "兰州",
	"西宁", "银川", "乌鲁木齐", "海口", "三亚",
}

func isAddressSuffix(s string) bool {
	for _, suf := range provinceCitySuffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// Utilities
// ─────────────────────────────────────────────────────────────────────────────

// removeSegment removes the first occurrence of seg from buf (case-insensitive)
// and collapses any resulting whitespace runs.
func removeSegment(buf, seg string) string {
	if seg == "" {
		return buf
	}
	lowerBuf := strings.ToLower(buf)
	lowerSeg := strings.ToLower(seg)
	idx := strings.Index(lowerBuf, lowerSeg)
	if idx == -1 {
		return buf
	}
	before := strings.TrimSpace(buf[:idx])
	after := strings.TrimSpace(buf[idx+len(seg):])
	return spaceRe.ReplaceAllString(strings.TrimSpace(before+" "+after), " ")
}
