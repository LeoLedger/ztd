package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/your-name/address-parse/internal/model"
)

// Package-level precompiled regexes — safe to compile once at init.
var (
	_                          = model.ParseResponse{} // force model import
	whitespaceRe               = regexp.MustCompile(`\s+`)
	punctuationRe              = regexp.MustCompile(`[，、；：？！…—·‛‟「」（）【】『』〔〕〖〗〈〉《》\t\n\r,;]+`)
	parenRe = regexp.MustCompile(`[（(]([^）)]+)[）)]`) // annotation parens: capture content to re-insert
	buildingRe                 = regexp.MustCompile(`大厦(\d+楼)`)
	cityAreaRe                 = regexp.MustCompile(`^市辖区`)
	districtGreedyRe           = regexp.MustCompile(`^(.+?)(区|县|市辖区|县级市)`)
)

type RuleEngine struct {
	provinces       map[string]string
	cities         map[string][]string
	districts      map[string]string
	provinceRe     *regexp.Regexp
	cityRe         *regexp.Regexp
	citySuffixRe   *regexp.Regexp
	streetSuffixRe *regexp.Regexp
}

func NewRuleEngine() *RuleEngine {
	provinces := map[string]string{
		"北京": "北京市", "天津": "天津市", "上海": "上海市", "重庆": "重庆市",
		"河北": "河北省", "山西": "山西省", "辽宁": "辽宁省", "吉林": "吉林省",
		"黑龙江": "黑龙江省", "江苏": "江苏省", "浙江": "浙江省", "安徽": "安徽省",
		"福建": "福建省", "江西": "江西省", "山东": "山东省", "河南": "河南省",
		"湖北": "湖北省", "湖南": "湖南省", "广东": "广东省", "海南": "海南省",
		"四川": "四川省", "贵州": "贵州省", "云南": "云南省", "陕西": "陕西省",
		"甘肃": "甘肃省", "青海": "青海省", "内蒙古": "内蒙古自治区", "广西": "广西壮族自治区",
		"西藏": "西藏自治区", "宁夏": "宁夏回族自治区", "新疆": "新疆维吾尔自治区",
		"香港": "香港特别行政区", "澳门": "澳门特别行政区", "台湾": "台湾省",
	}
	cities := map[string][]string{
		"广东省": {"广州", "深圳", "珠海", "东莞", "佛山", "中山", "惠州", "汕头", "江门", "湛江", "茂名", "肇庆", "梅州", "汕尾", "河源", "阳江", "清远", "韶关", "揭阳", "潮州", "云浮", "顺德", "增城", "从化", "乐昌", "南雄", "恩平", "鹤山", "开平", "廉江", "雷州", "吴川", "高州", "化州", "信宜", "阳春"},
		"北京市": {"北京"},
		"上海市": {"上海"},
		"天津市": {"天津"},
		"重庆市": {"重庆"},
		"浙江省": {"杭州", "宁波", "温州", "嘉兴", "湖州", "绍兴", "金华", "衢州", "舟山", "台州", "丽水", "余姚", "慈溪", "义乌", "东阳", "永康", "诸暨", "海宁", "平湖", "桐乡", "嘉善"},
		"江苏省": {"南京", "苏州", "无锡", "常州", "镇江", "扬州", "泰州", "南通", "盐城", "淮安", "连云港", "徐州", "宿迁", "昆山", "常熟", "张家港", "太仓", "江阴", "宜兴", "邳州", "新沂", "溧阳", "句容", "丹阳", "扬中", "靖江", "兴化", "泰兴"},
		"安徽省": {"合肥", "芜湖", "蚌埠", "淮南", "马鞍山", "淮北", "铜陵", "安庆", "黄山", "阜阳", "宿州", "滁州", "六安", "宣城", "池州", "亳州"},
		"福建省": {"福州", "厦门", "泉州", "漳州", "莆田", "宁德", "三明", "南平", "龙岩"},
		"江西省": {"南昌", "景德镇", "九江", "赣州", "吉安", "宜春", "抚州", "上饶", "鹰潭", "萍乡", "新余"},
		"山东省": {"济南", "青岛", "淄博", "枣庄", "东营", "烟台", "潍坊", "济宁", "泰安", "威海", "日照", "临沂", "德州", "聊城", "滨州", "菏泽", "滕州", "龙口", "莱阳", "蓬莱", "招远", "栖霞", "海阳", "青州", "诸城", "寿光", "安丘", "高密", "昌邑"},
		"河南省": {"郑州", "开封", "洛阳", "平顶山", "安阳", "鹤壁", "新乡", "焦作", "濮阳", "许昌", "漯河", "三门峡", "南阳", "商丘", "信阳", "周口", "驻马店", "济源"},
		"湖北省": {"武汉", "黄石", "十堰", "宜昌", "襄阳", "鄂州", "荆门", "孝感", "荆州", "黄冈", "咸宁", "随州", "恩施", "仙桃", "潜江", "天门", "神农架"},
		"湖南省": {"长沙", "株洲", "湘潭", "衡阳", "邵阳", "岳阳", "常德", "张家界", "益阳", "郴州", "永州", "怀化", "娄底", "湘西"},
		"四川省": {"成都", "自贡", "攀枝花", "泸州", "德阳", "绵阳", "广元", "遂宁", "内江", "乐山", "南充", "眉山", "宜宾", "广安", "达州", "雅安", "巴中", "资阳", "阿坝", "甘孜", "凉山"},
		"贵州省": {"贵阳", "遵义", "六盘水", "安顺", "毕节", "铜仁", "黔西南", "黔东南", "黔南"},
		"云南省": {"昆明", "曲靖", "玉溪", "保山", "昭通", "丽江", "普洱", "临沧", "楚雄", "红河", "文山", "西双版纳", "大理", "德宏", "怒江", "迪庆"},
		"陕西省": {"西安", "铜川", "宝鸡", "咸阳", "渭南", "延安", "汉中", "榆林", "安康", "商洛"},
		"甘肃省": {"兰州", "嘉峪关", "金昌", "白银", "天水", "武威", "张掖", "平凉", "酒泉", "庆阳", "定西", "陇南", "临夏", "甘南"},
		"青海省": {"西宁", "海东", "海北", "黄南", "海南", "果洛", "玉树", "海西"},
		"内蒙古自治区": {"呼和浩特", "包头", "乌海", "赤峰", "通辽", "鄂尔多斯", "呼伦贝尔", "巴彦淖尔", "乌兰察布", "兴安", "锡林郭勒", "阿拉善"},
		"广西壮族自治区": {"南宁", "柳州", "桂林", "梧州", "北海", "防城港", "钦州", "贵港", "玉林", "百色", "贺州", "河池", "来宾", "崇左"},
		"新疆维吾尔自治区": {"乌鲁木齐", "克拉玛依", "吐鲁番", "哈密"},
		"西藏自治区": {"拉萨", "日喀则", "昌都", "林芝", "山南", "那曲", "阿里"},
		"宁夏回族自治区": {"银川", "石嘴山", "吴忠", "固原", "中卫"},
		"海南省": {"海口", "三亚", "三沙", "儋州"},
		"黑龙江省": {"哈尔滨", "齐齐哈尔", "鸡西", "鹤岗", "双鸭山", "大庆", "伊春", "佳木斯", "七台河", "牡丹江", "黑河", "绥化"},
		"吉林省": {"长春", "吉林", "四平", "辽源", "通化", "白山", "松原", "白城", "延边"},
		"辽宁省": {"沈阳", "大连", "鞍山", "抚顺", "本溪", "丹东", "锦州", "营口", "阜新", "辽阳", "盘锦", "铁岭", "朝阳", "葫芦岛"},
		"山西省": {"太原", "大同", "阳泉", "长治", "晋城", "朔州", "晋中", "运城", "忻州", "临汾", "吕梁"},
		"河北省": {"石家庄", "唐山", "秦皇岛", "邯郸", "邢台", "保定", "张家口", "承德", "沧州", "廊坊", "衡水"},
	}

	provincePatterns := make([]string, 0, len(provinces)*2)
	for abbr, full := range provinces {
		provincePatterns = append(provincePatterns, regexp.QuoteMeta(full), regexp.QuoteMeta(abbr))
	}
	provinceRe := regexp.MustCompile(fmt.Sprintf(`^(%s)`, strings.Join(provincePatterns, "|")))

	cityPatterns := []string{}
	for _, cityList := range cities {
		for _, city := range cityList {
			cityPatterns = append(cityPatterns, regexp.QuoteMeta(city))
		}
	}
	cityPatternsRe := regexp.MustCompile(fmt.Sprintf(`^(%s)`, strings.Join(cityPatterns, "|")))

	return &RuleEngine{
		provinces:       provinces,
		cities:         cities,
		districts:      buildDistrictMap(),
		provinceRe:     provinceRe,
		cityRe:         cityPatternsRe,
		citySuffixRe:   regexp.MustCompile(`^(.+?)(市|地区)`),
		streetSuffixRe: regexp.MustCompile(`^([^\s\d\p{P}]{1,20}?)(街道|镇|乡)`),
	}
}

// buildDistrictMap creates a map from known district abbreviations to their full names.
func buildDistrictMap() map[string]string {
	m := map[string]string{
		"南山": "南山区", "福田": "福田区", "罗湖": "罗湖区", "盐田": "盐田区",
		"宝安": "宝安区", "龙岗": "龙岗区", "龙华": "龙华区", "坪山": "坪山区",
		"光明": "光明区", "大鹏": "大鹏新区",
		"南山区": "南山区",
		"天河": "天河区", "越秀": "越秀区", "荔湾": "荔湾区", "海珠": "海珠区",
		"白云": "白云区", "黄埔": "黄埔区", "番禺": "番禺区", "花都": "花都区",
		"南沙": "南沙区", "增城": "增城区", "从化": "从化区",
		"朝阳": "朝阳区", "海淀": "海淀区", "东城": "东城区", "西城": "西城区",
		"丰台": "丰台区", "石景山": "石景山区", "通州": "通州区", "顺义": "顺义区",
		"大兴": "大兴区", "房山": "房山区", "昌平": "昌平区", "怀柔": "怀柔区",
		"平谷": "平谷区", "门头沟": "门头沟区", "密云": "密云区", "延庆": "延庆区",
		"浦东": "浦东新区", "黄浦": "黄浦区", "静安": "静安区", "徐汇": "徐汇区",
		"长宁": "长宁区", "普陀": "普陀区", "虹口": "虹口区", "杨浦": "杨浦区",
		"闵行": "闵行区", "宝山": "宝山区", "嘉定": "嘉定区", "金山": "金山区",
		"松江": "松江区", "青浦": "青浦区", "奉贤": "奉贤区", "崇明": "崇明区",
		"西湖": "西湖区", "上城": "上城区", "拱墅": "拱墅区", "下城": "下城区",
		"滨江": "滨江区", "萧山": "萧山区", "余杭": "余杭区", "临平": "临平区",
		"钱塘": "钱塘区", "富阳": "富阳区", "临安": "临安区", "桐庐": "桐庐县",
		"建德": "建德市", "淳安": "淳安县",
		"玄武": "玄武区", "秦淮": "秦淮区", "建邺": "建邺区", "鼓楼": "鼓楼区",
		"浦口": "浦口区", "栖霞": "栖霞区", "雨花台": "雨花台区", "江宁": "江宁区",
		"六合": "六合区", "溧水": "溧水区", "高淳": "高淳区",
		"江岸": "江岸区", "江汉": "江汉区", "硚口": "硚口区", "汉阳": "汉阳区",
		"武昌": "武昌区", "青山": "青山区", "洪山": "洪山区", "东西湖": "东西湖区",
		"汉南": "汉南区", "蔡甸": "蔡甸区", "江夏": "江夏区", "黄陂": "黄陂区",
		"新洲": "新洲区",
		"锦江": "锦江区", "青羊": "青羊区", "金牛": "金牛区", "武侯": "武侯区",
		"成华": "成华区", "龙泉驿": "龙泉驿区", "青白江": "青白江区", "新都": "新都区",
		"温江": "温江区", "双流": "双流区", "郫都": "郫都区", "大邑": "大邑县",
		"蒲江": "蒲江县", "新津": "新津区", "简阳": "简阳市", "都江堰": "都江堰市",
		"彭州": "彭州市", "邛崃": "邛崃市", "崇州": "崇州市",
		"新城": "新城区", "碑林": "碑林区", "莲湖": "莲湖区", "灞桥": "灞桥区",
		"未央": "未央区", "雁塔": "雁塔区", "阎良": "阎良区", "临潼": "临潼区",
		"长安": "长安区", "高陵": "高陵区", "鄠邑": "鄠邑区", "蓝田": "蓝田县",
		"周至": "周至县",
		"和平": "和平区", "河东": "河东区", "河西": "河西区", "南开": "南开区",
		"河北": "河北区", "红桥": "红桥区", "滨海": "滨海新区", "宝坻": "宝坻区",
		"武清": "武清区", "蓟州": "蓟州区", "宁河": "宁河区", "静海": "静海区",
		"万州": "万州区", "渝中": "渝中区", "江北": "江北区", "沙坪坝": "沙坪坝区",
		"九龙坡": "九龙坡区", "南岸": "南岸区", "北碚": "北碚区", "渝北": "渝北区",
		"巴南": "巴南区", "涪陵": "涪陵区", "长寿": "长寿区", "璧山": "璧山区",
		"合川": "合川区", "永川": "永川区", "江津": "江津区", "綦江": "綦江区",
		"大足": "大足区", "铜梁": "铜梁区", "潼南": "潼南区", "荣昌": "荣昌区",
		"开州": "开州区", "梁平": "梁平区",
	}
	return m
}

// isCJK returns true if rune r is a CJK Unified Ideographs character.
func isCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// endsWithDistrictSuffix returns true if candidate ends with a valid district suffix.
func endsWithDistrictSuffix(candidate string) bool {
	return strings.HasSuffix(candidate, "区") ||
		strings.HasSuffix(candidate, "县") ||
		strings.HasSuffix(candidate, "市辖区") ||
		strings.HasSuffix(candidate, "县级市")
}

// scanBackDistrict scans the byte range s[:startPos] for a valid district suffix.
// It uses strings.LastIndex to find the last occurrence of 区/县/etc. at a valid
// UTF-8 character boundary. A valid candidate must have at least 2 CJK characters.
// Returns the matched district text or "".
func scanBackDistrict(s string, startPos int) string {
	if startPos <= 0 {
		return ""
	}
	search := s[:startPos]

	// First check: does the entire search space end with a valid district suffix?
	if len(search) >= 2 && endsWithDistrictSuffix(search) {
		runes := []rune(search)
		cnCount := 0
		for _, r := range runes {
			if isCJK(r) {
				cnCount++
			}
		}
		if cnCount >= 2 {
			return search
		}
	}

	// Find the last occurrence of 区 or 县 at a proper UTF-8 character boundary.
	// Use LastIndex with each suffix to avoid cutting multi-byte characters.
	for _, suffix := range []string{"市辖区", "县级市", "区", "县"} {
		idx := strings.LastIndex(search, suffix)
		if idx < 0 {
			continue
		}
		candidate := search[:idx+len(suffix)]
		if endsWithDistrictSuffix(candidate) {
			runes := []rune(candidate)
			cnCount := 0
			for _, r := range runes {
				if isCJK(r) {
					cnCount++
				}
			}
			if cnCount >= 2 {
				return candidate
			}
		}
	}
	return ""
}

// expandDistrict expands a bare district abbreviation (e.g. "南山") to its full
// form ("南山区") using the district map.
func (e *RuleEngine) expandDistrict(name string) string {
	if full, ok := e.districts[name]; ok {
		return full
	}
	return name
}

// tryExpandBareDistrict checks if remainder starts with a known bare district
// abbreviation followed by more address content. Returns (fullDistrict, rest).
// rejectParkFollow: if true, reject when abbreviation is immediately followed by
// a park keyword (科技园/创业园/etc.) — those are S1 territory.
// This function does NOT reject when followed by another district abbreviation.
func (e *RuleEngine) tryExpandBareDistrict(remainder string, rejectParkFollow bool) (string, string) {
	best := ""
	bestLen := 0
	for abbr, full := range e.districts {
		if !strings.HasPrefix(remainder, abbr) {
			continue
		}
		after := remainder[len(abbr):]
		if after == "" {
			continue
		}
		if rejectParkFollow {
			parkKeywords := []string{"科技园", "创业园", "工业园", "产业园区", "高新技术", "经济开发", "保税区"}
			followedByPark := false
			for _, kw := range parkKeywords {
				if strings.HasPrefix(after, kw) {
					followedByPark = true
					break
				}
			}
			if followedByPark {
				continue
			}
		}
		if len(abbr) > bestLen {
			best = full
			bestLen = len(abbr)
		}
	}
	if best != "" {
		return best, remainder[bestLen:]
	}
	return "", remainder
}

func (e *RuleEngine) Parse(address string) (*model.ParseResponse, bool) {
	addr := strings.TrimSpace(address)
	if addr == "" {
		return nil, false
	}

	result := &model.ParseResponse{}
	remainder := addr

	province, provOk := e.extractProvince(&remainder)
	if provOk {
		result.Province = province
	}

	city, cityOk := e.extractCity(result.Province, &remainder)
	if cityOk {
		result.City = city
	}

	district, distOk := e.extractDistrict(&remainder)
	if distOk {
		result.District = district
	}

	street, streetOk := e.extractStreet(&remainder)
	if streetOk {
		result.Street = street
	}

	result.Detail = refineDetail(strings.TrimSpace(remainder))

	if result.Province != "" || result.City != "" || result.District != "" {
		result.FullAddr = buildFullAddress(result)
		return result, true
	}

	return nil, false
}

func (e *RuleEngine) extractProvince(remainder *string) (string, bool) {
	matches := e.provinceRe.FindStringSubmatch(*remainder)
	if len(matches) > 1 {
		matched := matches[1]
		for abbr, full := range e.provinces {
			if abbr == matched || full == matched {
				*remainder = strings.TrimSpace(e.provinceRe.ReplaceAllString(*remainder, ""))
				return full, true
			}
		}
	}
	return "", false
}

func (e *RuleEngine) extractCity(province string, remainder *string) (string, bool) {
	*remainder = strings.TrimSpace(*remainder)

	if province != "" {
		if cityList, ok := e.cities[province]; ok {
			for _, city := range cityList {
				if strings.HasPrefix(*remainder, city) {
					rest := strings.TrimPrefix(*remainder, city)
					rest = strings.TrimPrefix(rest, "市")
					*remainder = strings.TrimSpace(rest)
					return city, true
				}
			}
			return "", false
		}
	}

	if e.cityRe.MatchString(*remainder) {
		matches := e.cityRe.FindStringSubmatch(*remainder)
		if len(matches) > 1 {
			city := matches[1]
			rest := e.cityRe.ReplaceAllString(*remainder, "")
			rest = strings.TrimPrefix(rest, "市")
			*remainder = strings.TrimSpace(rest)
			return city, true
		}
	}

	m := e.citySuffixRe.FindStringSubmatch(*remainder)
	if len(m) > 1 {
		*remainder = e.citySuffixRe.ReplaceAllString(*remainder, "")
		return m[1], true
	}

	return "", false
}

func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func (e *RuleEngine) extractDistrict(remainder *string) (string, bool) {
	*remainder = strings.TrimSpace(*remainder)
	*remainder = cityAreaRe.ReplaceAllString(*remainder, "")
	*remainder = strings.TrimSpace(*remainder)
	if *remainder == "" {
		return "", false
	}

	// Strategy 1: park keyword present → scan backwards for district suffix.
	parkKeywords := []string{"科技园", "创业园", "工业园", "产业园区", "高新技术", "经济开发", "保税区"}
	parkStart := -1
	for _, kw := range parkKeywords {
		idx := strings.Index(*remainder, kw)
		if idx != -1 && (parkStart == -1 || idx < parkStart) {
			parkStart = idx
		}
	}

	if parkStart != -1 {
		// S1a: scan backwards for district suffix before the park keyword.
		district := scanBackDistrict(*remainder, parkStart)
		if district != "" {
			afterDistrict := (*remainder)[len(district):]
			trimmed := strings.TrimSpace(afterDistrict)
			// Only accept if there is substantive street/detail content after the district.
			// Reject cases where the suffix looks like more address text (e.g. "南山区科技园"
			// → district="南山区南山区" is wrong because "科技园" follows, not street/detail).
			// Known district/park prefixes in the remainder after the district → invalid.
			hasBadSuffix := false
			for kw := range map[string]bool{"科技园": true, "创业园": true, "工业园": true, "产业园区": true, "高新技术": true} {
				if strings.HasPrefix(trimmed, kw) {
					hasBadSuffix = true
					break
				}
			}
			// Check for known district prefixes (e.g. "南山区南山区科技园")
			for abbr := range map[string]bool{"南山": true, "福田": true, "罗湖": true, "宝安": true, "龙岗": true} {
				if len(trimmed) >= 2 && len(abbr) <= len(trimmed) && trimmed[:len(abbr)] == abbr && string(trimmed[len(abbr)]) == "区" {
					hasBadSuffix = true
					break
				}
			}
			if !hasBadSuffix && len(trimmed) > 0 {
				normalized := e.expandDistrict(district)
				runeLen := utf8.RuneCountInString(district)
				remRunes := []rune(*remainder)
				*remainder = strings.TrimSpace(string(remRunes[runeLen:]))
				return normalized, true
			}
		}

		// S1b: no district suffix before park. Expand bare district abbreviation only when
		// there is substantive street or detail content after the abbreviation.
		normalized, rest := e.tryExpandBareDistrict(*remainder, false)
		if normalized != "" {
			trimmed := strings.TrimSpace(rest)
			// Check if there's substantive street/detail content (not just park keywords).
			// If the trimmed remainder starts with a park keyword, it only qualifies if there's
			// additional street detail AFTER the park keyword (e.g. "工业园大新路" has street detail).
			hasStreetDetail := false
			// Street/detail indicators: road names, numbers, building names
			for _, kw := range []string{"大道", "路", "巷", "号", "栋", "座", "楼", "室", "大厦", "广场", "花园", "中心"} {
				if strings.Contains(trimmed, kw) {
					hasStreetDetail = true
					break
				}
			}
			// Also accept if trimmed has mixed content (letters + numbers, etc.)
			if !hasStreetDetail {
				for _, c := range trimmed {
					if (c >= 'A' && c <= 'z') || (c >= '0' && c <= '9') {
						hasStreetDetail = true
						break
					}
				}
			}
			if hasStreetDetail {
				*remainder = rest
				return normalized, true
			}
		}
		// IMPORTANT: do NOT fall through to S2. If we had a park keyword and
		// couldn't find a district, return empty to prevent S2 from matching
		// districts that appear AFTER the park keyword.
		return "", false
	}

	// Strategy 2: no park keyword → find rightmost district suffix anywhere.
	// Reject if only park keywords or known district prefixes follow.
	district2 := scanBackDistrict(*remainder, len(*remainder))
	if district2 != "" {
		afterDistrict := (*remainder)[len(district2):]
		trimmed := strings.TrimSpace(afterDistrict)
		hasBadSuffix := false
		for kw := range map[string]bool{"科技园": true, "创业园": true, "工业园": true, "产业园区": true, "高新技术": true} {
			if strings.HasPrefix(trimmed, kw) {
				hasBadSuffix = true
				break
			}
		}
		if !hasBadSuffix && len(trimmed) > 0 {
			normalized := e.expandDistrict(district2)
			runeLen := utf8.RuneCountInString(district2)
			remRunes := []rune(*remainder)
			*remainder = strings.TrimSpace(string(remRunes[runeLen:]))
			return normalized, true
		}
	}

	// Strategy 3: greedy regex for literal 区/县/市辖区/县级市 suffix.
	m := districtGreedyRe.FindStringSubmatch(*remainder)
	if len(m) > 1 && len(m[1]) >= 1 {
		candidate := m[1] + m[2]
		normalized := e.expandDistrict(candidate)
		runeLen := utf8.RuneCountInString(candidate)
		remRunes := []rune(*remainder)
		*remainder = strings.TrimSpace(string(remRunes[runeLen:]))
		return normalized, true
	}

	// Strategy 4: expand bare district abbreviations at the start.
	// Used as final fallback for cases like "深圳南山大学城创业园" where
	// "南山" is known but no 区/县 suffix exists.
	normalized, rest := e.tryExpandBareDistrict(*remainder, false)
	if normalized != "" {
		*remainder = rest
		return normalized, true
	}

	return "", false
}

func (e *RuleEngine) extractStreet(remainder *string) (string, bool) {
	*remainder = strings.TrimSpace(*remainder)
	if *remainder == "" {
		return "", false
	}

	// 1. Match street/town/village suffixes at the start.
	if m := e.streetSuffixRe.FindStringSubmatch(*remainder); len(m) > 2 {
		prefix := m[1]
		suffix := m[2]
		// Strip leading district/unit marker included in prefix.
		// e.g. "南山区粤海街道" → strip "南山区" → "粤海"
		// Must use LastIndex to avoid cutting UTF-8 multi-byte characters in half.
		if idx := strings.LastIndex(prefix, "区"); idx >= 0 {
			prefix = strings.TrimSpace(prefix[idx+len("区"):])
		} else if idx := strings.LastIndex(prefix, "县"); idx >= 0 {
			prefix = strings.TrimSpace(prefix[idx+len("县"):])
		}
		if prefix != "" {
			*remainder = strings.TrimSpace(e.streetSuffixRe.ReplaceAllString(*remainder, ""))
			return prefix + suffix, true
		}
	}

	// 2. Match street suffix anywhere (greedy, rightmost).
	for _, suffix := range []string{"街道", "镇", "乡"} {
		idx := strings.LastIndex(*remainder, suffix)
		if idx > 0 {
			prefix := strings.TrimSpace((*remainder)[:idx])
			runes := []rune(prefix)
			cnCount := 0
			for _, r := range runes {
				if isCJK(r) {
					cnCount++
				}
			}
			if cnCount >= 2 {
				street := prefix + suffix
				*remainder = strings.TrimSpace((*remainder)[idx+len(suffix):])
				return street, true
			}
		}
	}

	// 3. Match road names.
	roadNameSuffixes := []string{"大道", "路", "巷", "社区"}
	type roadMatch struct{ suffix string; byteIdx int }
	var candidates []roadMatch

	for _, s := range roadNameSuffixes {
		offset := 0
		for {
			idx := strings.Index((*remainder)[offset:], s)
			if idx < 0 {
				break
			}
			absIdx := offset + idx
			valid := true

			if absIdx > 0 {
				prevByte := (*remainder)[absIdx-1]
				for _, s2 := range roadNameSuffixes {
					if len(s2) > 0 && prevByte == s2[0] {
						valid = false
						break
					}
				}
			}

			if valid && s == "道" && absIdx > 0 {
				prevRune := rune((*remainder)[absIdx-1])
				if prevRune == '城' {
					valid = false
				}
			}
			if valid && s == "路" && absIdx > 0 {
				prevRune := rune((*remainder)[absIdx-1])
				if prevRune == '城' {
					valid = false
				}
			}

			if valid {
				candidates = append(candidates, roadMatch{s, absIdx})
			}
			offset = absIdx + len(s)
		}
	}

	if len(candidates) == 0 {
		return "", false
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.byteIdx > best.byteIdx {
			best = c
		}
	}

	maxPrefixRunes := 8
	runes := []rune(*remainder)
	suffixRuneIdx := utf8.RuneCountInString((*remainder)[:best.byteIdx])
	if suffixRuneIdx > len(runes) {
		suffixRuneIdx = len(runes)
	}

	countCN := func(s string) int {
		n := 0
		for _, r := range s {
			if r >= 0x4e00 && r <= 0x9fff {
				n++
			}
		}
		return n
	}

	for startRune := suffixRuneIdx; startRune >= 0; startRune-- {
		offset := suffixRuneIdx - startRune
		if offset > maxPrefixRunes && startRune > 0 {
			break
		}
		prefix := string(runes[startRune:suffixRuneIdx])
		if countCN(prefix) >= 2 {
			restByteIdx := best.byteIdx + len(best.suffix)
			*remainder = strings.TrimSpace((*remainder)[restByteIdx:])
			return prefix + best.suffix, true
		}
	}

	return "", false
}

func buildFullAddress(r *model.ParseResponse) string {
	parts := []string{}
	if r.Province != "" {
		parts = append(parts, r.Province)
	}
	if r.City != "" {
		parts = append(parts, r.City)
	}
	if r.District != "" {
		parts = append(parts, r.District)
	}
	if r.Street != "" {
		parts = append(parts, r.Street)
	}
	if r.Detail != "" {
		parts = append(parts, r.Detail)
	}
	return strings.Join(parts, " ")
}

func HashAddress(address string) string {
	h := sha256.Sum256([]byte(address))
	return hex.EncodeToString(h[:])
}

func SerializeResponse(r *model.ParseResponse) (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func DeserializeResponse(data string) (*model.ParseResponse, error) {
	var r model.ParseResponse
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// NormalizeText normalizes user input: full-width→half-width, unifies separators, strips emoji.
// All runs of whitespace (spaces, tabs, newlines \n\r, mixed \r\n) are collapsed to a single space.
func NormalizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// First: collapse all whitespace runs (including lone \n/\t/\r) to a single space.
	s = whitespaceRe.ReplaceAllString(s, " ")
	// Process characters on the already-spaced string.
	var result []rune
	for _, r := range s {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			result = append(result, r-0xFEE0)
		case r == 0x200B:
			continue
		case unicode.Is(unicode.So, r) || unicode.Is(unicode.Co, r):
			continue
		case r >= 0x1F300 && r <= 0xF9FF:
			continue
		case r >= 0x2600 && r <= 0x26FF && !strings.ContainsRune("☀☁☂☃℃℉", r):
			continue
		case unicode.IsControl(r):
			continue
		default:
			result = append(result, r)
		}
	}
	s = string(result)
	s = punctuationRe.ReplaceAllString(s, " ")
	// Collapse any new spaces introduced by punctuation replacement, then final trim.
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func refineDetail(remainder string) string {
	remainder = strings.Trim(remainder, " \t\n\r,.，、；：")
	return remainder
}

// Preprocess removes noise that would interfere with address parsing.
func Preprocess(address string) string {
	s := NormalizeText(address)
	// Strip annotation parentheses, preserving the content inside.
	s = parenRe.ReplaceAllString(s, " $1 ")
	s = buildingRe.ReplaceAllString(s, "大厦 $1")
	s = whitespaceRe.ReplaceAllString(s, " ")
	s = stripCJKSpaces(s)
	return strings.TrimSpace(s)
}

// stripCJKSpaces removes spaces that appear between two CJK characters.
// This corrects input like "深 圳市" → "深圳市" and "南 山区" → "南山区".
// Spaces between a CJK char and a non-CJK char (e.g. "深圳市 88号") are preserved.
func stripCJKSpaces(s string) string {
	var result []rune
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == ' ' && i > 0 && i < len(runes)-1 {
			prev := runes[i-1]
			next := runes[i+1]
			if isCJK(prev) && isCJK(next) {
				continue // skip space between two CJK chars
			}
		}
		result = append(result, runes[i])
	}
	return string(result)
}
