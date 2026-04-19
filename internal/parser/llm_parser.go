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

func (p *LLMParser) Parse(ctx context.Context, address string) (*model.ParseResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("LLM API key not configured")
	}

	prompt := fmt.Sprintf(`你是一个专业的地址解析系统。请将以下地址解析为结构化JSON，只返回JSON，不要任何其他文字。
JSON字段：province(省，如广东省)、city(市，如深圳市)、district(区/县，如南山区)、street(街道，如桃源街道)、detail(详细地址)

输入地址：%s

规则：
1. 如果地址中缺少省份或城市信息，应根据中国行政区划知识进行合理补全。例如"南山区桃源街道"应补全为 province=广东省、city=深圳市、district=南山区、street=桃源街道
2. 补全时应根据地址的语义和常见行政区划知识推断，不能凭空编造不确定的信息
3. 如果输入地址存在错别字、异体字、格式混乱等错误，应在对应字段中进行合理修正
4. 详细地址部分（门牌号、楼栋、单元等）必须保留原样，不做修改

只输出JSON，不要解释。`, address)

	reqBody := qwenRequest{
		Model: p.model,
		Messages: []qwenMessage{
			{Role: "system", Content: "你是一个专业的地址解析系统。如果地址缺少省份或城市，应根据中国行政区划知识合理补全；同时修正错别字和格式错误。详细地址部分必须保留原样。允许部分字段为空。只返回JSON格式，不要任何解释。"},
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		result, err := p.doParse(ctx, jsonBody)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("LLM parse failed after retries: %w", lastErr)
}

func (p *LLMParser) doParse(ctx context.Context, jsonBody []byte) (*model.ParseResponse, error) {
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

	return &result, nil
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
