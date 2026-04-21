package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/internal/model"
	"github.com/your-name/address-parse/internal/parser"
)

const (
	MethodCache = "cache"
	MethodRule  = "rule"
	MethodLLM   = "llm"
)

type ParserService struct {
	ruleEngine *parser.RuleEngine
	llmParser  *parser.LLMParser
	cache      *parser.Cache
}

func NewParserService(cfg *config.Config, redisClient *redis.Client) *ParserService {
	var cache *parser.Cache
	if redisClient != nil {
		cache, _ = parser.NewCache(cfg.Redis.URL)
	}

	return &ParserService{
		ruleEngine: parser.NewRuleEngine(),
		llmParser:  parser.NewLLMParser(cfg),
		cache:      cache,
	}
}

type ParseResult struct {
	Response    *model.ParseResponse
	Method      string
	ParseTimeMs int
}

// Parse extracts structured address fields from raw free-text input.
// Strategy: LLM (primary) — parses the raw text directly from scratch, best at
// out-of-order, missing-field, and mixed-format inputs.
// RuleEngine (fallback) — used only when LLM is unavailable or fails.
// Results are cached for 24h keyed on the raw original text.
func (s *ParserService) Parse(ctx context.Context, req *model.RawFields) (*ParseResult, error) {
	start := time.Now()

	// rawText is the primary text for LLM parsing.
	// For structured input (legacy API), assemble a readable text from the fields
	// so LLM can parse all fields together rather than from an empty OriginalText.
	rawText := req.OriginalText
	if rawText == "" {
		parts := []string{}
		if req.Name != "" {
			parts = append(parts, req.Name)
		}
		if req.Phone != "" {
			parts = append(parts, req.Phone)
		}
		if req.Company != "" {
			parts = append(parts, req.Company)
		}
		if req.Address != "" {
			parts = append(parts, req.Address)
		}
		rawText = joinNonEmpty(parts, " ")
	}
	if rawText == "" {
		return nil, fmt.Errorf("address is required")
	}

	cacheKey := "addr:" + parser.HashAddress(rawText)

	// Try cache first.
	if s.cache != nil {
		cached, err := s.cache.Get(ctx, cacheKey)
		if err == nil && cached != "" {
			result, err := parser.DeserializeResponse(cached)
			if err == nil {
				return &ParseResult{
					Response:    result,
					Method:      MethodCache,
					ParseTimeMs: int(time.Since(start).Milliseconds()),
				}, nil
			}
		}
	}

	// Primary: LLM parses the raw text directly.
	if s.llmParser != nil {
		llmResult, err := s.llmParser.Parse(ctx, req)
		if err == nil {
			// Clean contaminated results: extract only the valid company/name portion.
			llmResult.Company = cleanCompany(llmResult.Company)
			llmResult.Name = cleanName(llmResult.Name, llmResult.Phone)

			if s.cache != nil {
				data, _ := parser.SerializeResponse(llmResult)
				_ = s.cache.Set(ctx, cacheKey, data, 24*time.Hour)
			}
			return &ParseResult{
				Response:    llmResult,
				Method:      MethodLLM,
				ParseTimeMs: int(time.Since(start).Milliseconds()),
			}, nil
		}
		// LLM failed; fall through to rule engine.
	}

	// Fallback: rule engine (only when LLM is unavailable or failed).
	// Use the address field directly — name/phone/company were already stripped by
	// ExtractFields in the handler layer and reassembled into req.Address.
	addr := parser.Preprocess(req.Address)
	if result, ok := s.ruleEngine.Parse(addr); ok {
		// Passthrough structured fields when present (e.g. from legacy API format).
		if req.Name != "" {
			result.Name = req.Name
		}
		if req.Phone != "" {
			result.Phone = req.Phone
		}
		if req.Company != "" {
			result.Company = req.Company
		}

		if s.cache != nil {
			data, _ := parser.SerializeResponse(result)
			_ = s.cache.Set(ctx, cacheKey, data, 24*time.Hour)
		}
		return &ParseResult{
			Response:    result,
			Method:      MethodRule,
			ParseTimeMs: int(time.Since(start).Milliseconds()),
		}, nil
	}

	return nil, fmt.Errorf("failed to parse address")
}

func joinNonEmpty(parts []string, sep string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// cleanCompany removes address fragments from a contaminated company field.
// Strategy: find the last space in the string, then take everything after it.
// This cleanly separates "桑泰大厦13楼1303室 智腾达科技" → "智腾达科技".
// The extracted suffix must look like a company name (ends with known marker,
// no heavy address markers inside).
func cleanCompany(company string) string {
	if company == "" {
		return ""
	}

	// Strategy 1: find the last space and take everything after it.
	if lastSpace := strings.LastIndex(company, " "); lastSpace >= 0 {
		result := strings.TrimSpace(company[lastSpace:])
		if result != "" && isCleanCompanySuffix(result) {
			return result
		}
	}

	// Strategy 2: no space — look for "室" (room number) as the boundary.
	if lastRoom := strings.LastIndex(company, "室"); lastRoom >= 0 && lastRoom < len(company)-1 {
		result := strings.TrimSpace(company[lastRoom+1:])
		if result != "" && isCleanCompanySuffix(result) {
			return result
		}
	}

	// Fallback: no clean split point. Only keep if the whole thing is short and clean.
	if len(company) <= 15 && !hasAddressMarkers(company) {
		return company
	}
	return ""
}

// isCleanCompanySuffix returns true if s looks like a company name.
// Accepts: starts with a known company marker, OR ends with a known marker
// AND has no province/city markers within it.
func isCleanCompanySuffix(s string) bool {
	prefixMarkers := []string{"有限公司", "股份有限公司", "集团有限公司",
		"公司", "集团", "科技有限公司", "科技", "Co.", "LTD"}
	for _, m := range prefixMarkers {
		if strings.HasPrefix(s, m) {
			return true
		}
	}
	suffixMarkers := []string{"有限公司", "公司", "集团", "科技"}
	for _, m := range suffixMarkers {
		if strings.HasSuffix(s, m) && !hasAddressMarkers(s) {
			return true
		}
	}
	return false
}

// hasAddressMarkers returns true if s contains multiple province/city/district markers,
// suggesting it's a geographic fragment rather than a company name.
func hasAddressMarkers(s string) bool {
	count := 0
	for _, m := range []string{"省", "市", "区", "县", "街道", "镇", "路", "号"} {
		if strings.Contains(s, m) {
			count++
		}
	}
	return count >= 2
}

// cleanName removes phone numbers from a potentially contaminated name field.
func cleanName(name, phone string) string {
	if name == "" {
		return ""
	}
	if phone != "" && (name == phone || strings.Contains(name, phone)) {
		return ""
	}
	digitCount := 0
	for _, c := range name {
		if c >= '0' && c <= '9' {
			digitCount++
		}
	}
	if digitCount >= 7 && digitCount == len([]rune(name)) {
		return ""
	}
	return name
}
