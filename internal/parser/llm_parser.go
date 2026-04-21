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
	extra := buildExtraContext(fields)

	systemPrompt := `你是一个专业的中国地址解析系统，负责将任意格式的地址文本解析为结构化字段。

## 你的职责：
1. 地址补全：根据中国行政区划知识，从地址文本中提取或合理推断省份、城市、区县。
2. 姓名/电话/公司识别：如果输入文本中包含联系人信息（姓名、电话）、公司名称，应一并提取。
3. 详细地址保留：门牌号、楼栋、单元、房间号等详细地址必须保留原始内容。
4. 允许字段为空：输入文本中缺失的字段可以为空字符串，但不要凭空编造。

## 严格禁止：
- 不要修改已提供的确切信息（如姓名、电话、公司名）。
- 不要凭空添加地址中不存在的详细信息。

## 输出格式：
仅返回标准 JSON 格式，无需任何解释。JSON 字段：name(联系人姓名)、phone(联系电话)、company(公司名称)、province(省)、city(市)、district(区/县)、street(街道)、detail(详细地址)`

	userPrompt := fmt.Sprintf(`请将以下地址文本解析为结构化JSON。

输入文本：%s
%s

要求：
- 仔细分析输入文本，将其中的姓名、电话、公司名称、省市县区街道、详细地址分别提取到对应JSON字段。
- 如果地址中缺少省份或城市，应根据中国行政区划知识进行合理推断补全。
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
