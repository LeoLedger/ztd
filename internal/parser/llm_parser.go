package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/internal/model"
)

type LLMParser struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

type qwenRequest struct {
	Model    string        `json:"model"`
	Messages []qwenMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type qwenMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type qwenResponse struct {
	Choices []qwenChoice `json:"choices"`
}

type qwenChoice struct {
	Message qwenMessage `json:"message"`
}

func NewLLMParser(cfg *config.Config) *LLMParser {
	return &LLMParser{
		apiKey:  cfg.LLM.APIKey,
		model:   cfg.LLM.Model,
		baseURL: cfg.LLM.BaseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *LLMParser) Parse(ctx context.Context, fields *model.RawFields) (*model.ParseResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("LLM API key not configured")
	}

	// Use the ORIGINAL full text as the primary source for LLM.
	// The pre-extracted fields (name/phone/company) are used as hints to confirm
	// the LLM's own extraction, not as the definitive answer.
	// This is critical for out-of-order input where "广东省深圳市... 智腾达科技"
	// would otherwise lose the company name after Preprocess strips CJK spaces.
	originalText := fields.OriginalText
	if originalText == "" {
		originalText = fields.Address
	}
	// Remove duplicated administrative region prefixes (e.g. "广东省惠州市广东省惠州市" → "广东省惠州市").
	originalText = DeduplicateAdministrativePrefix(originalText)
	extra := buildExtraContext(fields)

	systemPrompt := `你是一个专业的中国地址解析系统，负责将任意格式的地址文本解析为结构化字段。

## 你的职责：
1. 地址补全：根据中国行政区划知识，从地址文本中提取或合理推断省份、城市、区县。
2. 姓名/电话/公司识别：如果输入文本中包含联系人信息（姓名、电话）、公司名称，应一并提取。
3. 详细地址保留：门牌号、楼栋、单元、房间号等详细地址必须保留原始内容。
4. 允许字段为空：输入文本中缺失的字段可以为空字符串，但不要凭空编造。

## 区县推理规则（重要）：
district 字段必须填写「区」或「县」名称（如 龙华区、南山区、惠城区），不能填写街道名或社区名。
当地址文本中没有明确写出区县时，应根据以下线索推断：
- **街道→区映射（必须严格遵守）**：
  - 深圳市：观湖/观澜/龙华/民治/大浪/福城街道 → 龙华区；粤海/西丽/桃源/科技园 → 南山区；
    华富/香蜜湖/福田CBD/市民中心/竹子林/车公庙 → 福田区；东门/翠竹/国贸/莲塘 → 罗湖区；
    新安/西乡/福永/沙井/石岩 → 宝安区；坂田/布吉/龙城/横岗 → 龙岗区；
    盐田/海山/沙头角/梅沙 → 盐田区；光明/公明 → 光明区；坪山/坑梓 → 坪山区
  - 广州市：猎德/员村/棠下/天河南/石牌/林和 → 天河区；科学城/知识城 → 黄埔区；
    珠江新城/北京路 → 天河区；荔湾/陈家祠 → 荔湾区
  - 惠州市：河南岸/江北/桥西/桥东/龙丰/惠环/陈江/水口/小金口 → 惠城区；
    大亚湾/澳头/淡水 → 惠阳区
  - 东莞市：南城/东城/莞城 → 对应区；虎门/长安/厚街 → 滨海片区；松山湖 → 松山湖功能区
  - 佛山市：桂城/桂城街道 → 南海区
- **社区名/地名**：金湖社区/江北/桥西/桥东 → 惠城区（惠州市）；银湖/东门/翠竹 → 罗湖区（深圳市）
- **道路名**：梅观路/富士康 → 龙华区（深圳市）
- **重要**：如果输入文本中有"XX街道"但没有写明"XX区"，district 字段应根据以上映射填入对应的区（如 河南岸街道 → 惠城区），而不是街道名。如果确实无法推断则留空。

## 严格禁止：
- 不要修改已提供的确切信息（如姓名、电话、公司名）。
- 不要凭空添加地址中不存在的详细信息。

## 输出格式：
仅返回标准 JSON 格式，无需任何解释。JSON 字段：name(联系人姓名)、phone(联系电话)、company(公司名称)、province(省)、city(市)、district(区/县)、street(街道)、detail(详细地址)`

	userPrompt := fmt.Sprintf(`请将以下地址文本解析为结构化JSON。

输入文本：%s
%s

【示例解析】（学习以下例子，不要直接复制）：

示例1：
输入：深圳市腾讯科技有限公司，张三，13812345678，广东省惠州市惠城区河南岸街道金湖社区
解析结果：
{
  "name": "张三",
  "phone": "13812345678",
  "company": "深圳市腾讯科技有限公司",
  "province": "广东省",
  "city": "惠州市",
  "district": "惠城区",
  "street": "河南岸街道",
  "detail": "金湖社区"
}
分析：文本前半部分"深圳市腾讯科技有限公司，张三，13812345678"是联系信息；"广东省惠州市惠城区河南岸街道金湖社区"才是地址。地址中的"惠州市惠城区"明确给出了city和district，无需推理。

示例2（需要推理）：
输入：XX有限公司，李四，13900001111，广东省惠州市河南岸街道金湖社区张屋山一巷二号
解析结果：
{
  "name": "李四",
  "phone": "13900001111",
  "company": "XX有限公司",
  "province": "广东省",
  "city": "惠州市",
  "district": "惠城区",
  "street": "河南岸街道",
  "detail": "金湖社区张屋山一巷二号"
}
分析：地址中有"惠州市"但没有写明区。查表可知"河南岸街道"属于惠城区，故district填"惠城区"。

要求：
- 仔细分析输入文本，将其中的姓名、电话、公司名称、省市县区街道、详细地址分别提取到对应JSON字段。
- 地址判断规则：
  * 地址一定包含省/市/区/县/街道等行政区划关键词（省、市、区、县、街道、路、巷、号等）。
  * 如果文本中同时出现公司名和地址，公司名在前、地址在后（例如"XX公司，张三，138xxxx，广东省深圳市南山区..."），应将前半段识别为公司/人/电话，剩余部分识别为地址。
  * 如果地址中缺少省份或城市，应根据中国行政区划知识进行合理推断补全。
  * 如果地址中缺少区县，应根据街道名/社区名/道路名推断。注意：district字段必须填区/县名称（如 龙华区、惠城区），不能填街道名。
    示例：地址中有"观湖街道"→深圳市龙华区；"猎德街道"→广州市天河区；"河南岸街道"→惠州市惠城区。
- 如果存在错别字、异体字，应在对应字段中进行合理修正。
- 如果某字段在输入文本中缺失则留空，不要编造不存在的信息。
- 详细地址部分（门牌号、楼栋、单元、房间号等）必须保留原样。
- 直接输出JSON，不要任何解释文字。`, originalText, extra)

	reqBody := qwenRequest{
		Model: p.model,
		Messages: []qwenMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		result, err := p.doParse(ctx, jsonBody, fields, originalText)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("LLM parse failed after retries: %w", lastErr)
}

func (p *LLMParser) doParse(ctx context.Context, jsonBody []byte, fields *model.RawFields, originalText string) (*model.ParseResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API returned status %d", resp.StatusCode)
	}

	var qwenResp qwenResponse
	if err := json.NewDecoder(resp.Body).Decode(&qwenResp); err != nil {
		return nil, fmt.Errorf("failed to decode LLM response: %w", err)
	}

	if len(qwenResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	content := qwenResp.Choices[0].Message.Content
	content = cleanJSONResponse(content)

	var result model.ParseResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("LLM response not valid JSON: %w", err)
	}

	result.FullAddr = buildFullAddressFromLLM(&result)

	// Trust pre-extracted name/phone/company when available.
	// When they are empty (e.g. company appears after the address portion and
	// extractCompany fails to catch it), fall back to LLM's own extraction.
	if fields.Name != "" {
		result.Name = fields.Name
	}
	if fields.Phone != "" {
		result.Phone = fields.Phone
	}
	if fields.Company != "" {
		result.Company = fields.Company
	} else {
		// extractCompany failed (company appears after address or lacks a marker keyword).
		// Fallback: scan OriginalText for a valid company name substring.
		if originalText != "" {
			if candidate := findCompanyInText(originalText); candidate != "" {
				result.Company = candidate
			}
		}
		// If no candidate found, fall back to LLM's company only if clean.
		if result.Company == "" && !looksLikeAddressContaminated(result.Company) {
			// keep LLM's result
		} else if result.Company == "" {
			// already empty, nothing to do
		} else {
			// contaminated → try OriginalText scan one more time
			if candidate := findCompanyInText(originalText); candidate != "" {
				result.Company = candidate
			} else {
				result.Company = ""
			}
		}
	}

	return &result, nil
}

// findCompanyInText scans originalText for a company name ending with a known marker
// keyword. Returns the found company name or "". This is used as a fallback when
// extractCompany failed to find a company in the preprocessed address.
func findCompanyInText(text string) string {
	if text == "" {
		return ""
	}
	markers := []string{"有限公司", "股份有限公司", "责任公司", "集团有限公司",
		"公司", "集团", "科技", "Co.", "LTD", "Inc."}
	for _, marker := range markers {
		idx := strings.Index(text, marker)
		if idx > 1 {
			// Back up to find a reasonable starting position (max ~20 chars before marker).
			start := idx - 20
			if start < 0 {
				start = 0
			}
			candidate := strings.TrimSpace(text[start : idx+len(marker)])
			// Valid candidate must be at least 4 chars and not consist of address fragments.
			if len(candidate) >= 4 && !looksLikeAddressContaminated(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// looksLikeAddressContaminated returns true if s looks like a company name
// that was contaminated with address fragments (e.g. "广东省深圳市... 智腾达科技"
// ends up as a single company field with province/city in it).
// Detection heuristic: s contains a known province/city abbreviation or is
// suspiciously long (>= 30 chars suggests address+company merge).
func looksLikeAddressContaminated(s string) bool {
	if len(s) >= 30 {
		return true
	}
	contaminants := []string{
		"省", "市", "区", "县", "街道", "镇", "乡",
		"路", "号", "栋", "楼", "室", "单元",
		"大道", "广场", "大厦", "花园", "中心",
		"科技园", "工业园", "创业园",
	}
	count := 0
	for _, c := range contaminants {
		if strings.Contains(s, c) {
			count++
		}
	}
	// Contains two or more address markers → likely contaminated.
	return count >= 2
}

// cleanJSONResponse strips markdown fences and extracts the first JSON object.
func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)

	// Strip markdown code fences: ```json ... ``` or ``` ...
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) >= 2 {
			s = lines[1]
		}
		if strings.HasSuffix(s, "```") {
			s = s[:len(s)-3]
		}
	}

	// Find first '{' and last '}'
	start, end := -1, len(s)
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			start = i
			break
		}
	}
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '}' {
			end = i + 1
			break
		}
	}
	if start == -1 || start >= end {
		return s
	}
	return s[start:end]
}

func buildFullAddressFromLLM(r *model.ParseResponse) string {
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
	return joinNonEmpty(parts, "")
}

func joinNonEmpty(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// DeduplicateAdministrativePrefix trims all text before the LAST province/city pattern.
// This prevents a city name appearing in a company name (e.g. "深圳市智腾达公司，惠州...")
// from being matched before the actual address city (惠州).
// Also collapses multiple spaces between duplicated prefixes:
// "广东省惠州市 广东省惠州市 辰芊科技" → "广东省惠州市辰芊科技"
func DeduplicateAdministrativePrefix(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 12 {
		return s
	}

	// Collapse whitespace: first collapse runs of spaces to single spaces,
	// then handle any remaining double spaces.
	// Using strings.ReplaceAll (not regex) ensures all space characters are handled.
	normalized := strings.ReplaceAll(s, " ", " ")
	for strings.Contains(normalized, "  ") {
		normalized = strings.ReplaceAll(normalized, "  ", " ")
	}

	// Prefix patterns with actual byte lengths, longest-first.
	type pat struct{ p string; n int }
	patterns := []pat{
		{"新疆维吾尔自治区", 24}, {"内蒙古自治区", 18}, {"广西壮族自治区", 21},
		{"宁夏回族自治区", 21}, {"西藏自治区", 15},
		{"广东省深圳市", 18}, {"广东省广州市", 18}, {"广东省东莞市", 18},
		{"广东省佛山市", 18}, {"广东省惠州市", 18}, {"广东省珠海市", 18},
		{"广东省中山市", 18}, {"广东省汕头市", 18}, {"广东省江门市", 18},
		{"广东省湛江市", 18}, {"广东省茂名市", 18}, {"广东省肇庆市", 18},
		{"广东省梅州市", 18}, {"广东省汕尾市", 18}, {"广东省河源市", 18},
		{"广东省阳江市", 18}, {"广东省清远市", 18}, {"广东省韶关市", 18},
		{"广东省揭阳市", 18}, {"广东省潮州市", 18}, {"广东省云浮市", 18},
		{"浙江省杭州市", 18}, {"浙江省宁波市", 18}, {"浙江省温州市", 18},
		{"浙江省嘉兴市", 18}, {"浙江省湖州市", 18}, {"浙江省绍兴市", 18},
		{"浙江省金华市", 18}, {"浙江省衢州市", 18}, {"浙江省舟山市", 18},
		{"浙江省台州市", 18}, {"浙江省丽水市", 18},
		{"江苏省南京市", 18}, {"江苏省苏州市", 18}, {"江苏省无锡市", 18},
		{"江苏省常州市", 18}, {"江苏省镇江市", 18}, {"江苏省扬州市", 18},
		{"江苏省泰州市", 18}, {"江苏省南通市", 18}, {"江苏省盐城市", 18},
		{"江苏省淮安市", 18}, {"江苏省连云港市", 21}, {"江苏省徐州市", 18},
		{"江苏省宿迁市", 18},
		{"北京市", 9}, {"上海市", 9}, {"天津市", 9}, {"重庆市", 9},
		{"四川省成都市", 18}, {"湖北省武汉市", 18}, {"湖南省长沙市", 18},
		{"安徽省合肥市", 18}, {"福建省福州市", 18}, {"福建省厦门市", 18},
		{"山东省济南市", 18}, {"山东省青岛市", 18}, {"河南省郑州市", 18},
		{"河北省石家庄市", 21}, {"陕西省西安市", 18},
		{"黑龙江省", 12},
		{"广东省", 9}, {"浙江省", 9}, {"江苏省", 9},
		{"四川省", 9}, {"湖北省", 9}, {"湖南省", 9},
		{"安徽省", 9}, {"福建省", 9}, {"山东省", 9},
		{"河南省", 9}, {"河北省", 9}, {"陕西省", 9},
		{"江西省", 9}, {"山西省", 9}, {"辽宁省", 9},
		{"吉林省", 9}, {"海南省", 9},
		{"内蒙古", 9}, {"广西", 6}, {"新疆", 6},
		{"西藏", 6}, {"宁夏", 6},
	}

	// Strip consecutive pattern occurrences from the start.
	// e.g. "广东省惠州市广东省惠州市辰芊科技" → "广东省惠州市辰芊科技"
	// e.g. "北京市北京市东城区" → "北京市东城区"
	stripped := 0
	rest := normalized
	for {
		matched := false
		for _, pat := range patterns {
			if strings.HasPrefix(rest, pat.p) {
				stripped += len(pat.p)
				rest = rest[pat.n:]
				// Skip the "市" suffix if it follows immediately (e.g. "深圳市" strip "深圳市" not just "深圳")
				// and any whitespace
				rest = strings.TrimLeft(rest, " ")
				matched = true
				break
			}
		}
		if !matched {
			break
		}
	}
	if stripped > 0 {
		// Consecutive pattern(s) found at start. Keep the first one, strip the rest.
		// Determine what the first pattern was.
		firstLen := 0
		for _, pat := range patterns {
			if strings.HasPrefix(normalized, pat.p) {
				firstLen = len(pat.p)
				break
			}
		}
		// Return: first pattern + everything after all consecutive patterns
		return normalized[:firstLen] + rest
	}

	// No consecutive patterns at start. Check if a city appears within the string
	// (e.g. company name contains "深圳市" before the actual address "惠州...").
	// Find the LAST occurrence of any pattern.
	lastIdx := -1
	for _, pat := range patterns {
		idx := strings.LastIndex(normalized, pat.p)
		if idx > lastIdx {
			lastIdx = idx
		}
	}
	if lastIdx > 0 {
		return normalized[lastIdx:]
	}
	return s
}

// buildExtraContext formats pre-extracted fields as a hint block appended to the prompt.
func buildExtraContext(f *model.RawFields) string {
	parts := []string{}
	if f.Name != "" {
		parts = append(parts, "已提取姓名: "+f.Name)
	}
	if f.Phone != "" {
		parts = append(parts, "已提取电话: "+f.Phone)
	}
	if f.Company != "" {
		parts = append(parts, "已提取公司: "+f.Company)
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n参考信息（已从原文提取）：\n" + strings.Join(parts, "\n")
}
