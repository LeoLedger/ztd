package service

import (
	"context"
	"fmt"
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
		_ = cache // parser package uses its own client; re-use connection
	}

	return &ParserService{
		ruleEngine: parser.NewRuleEngine(),
		llmParser:  parser.NewLLMParser(cfg),
		cache:      cache,
	}
}

type ParseResult struct {
	Response   *model.ParseResponse
	Method     string
	ParseTimeMs int
}

func (s *ParserService) Parse(ctx context.Context, req *model.ParseRequest) (*ParseResult, error) {
	start := time.Now()

	if req.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	// Preprocess the raw address to remove noise (company names, separators, emoji)
	cleanedAddr := parser.Preprocess(req.Address)
	cacheKey := "addr:" + parser.HashAddress(cleanedAddr)

	// Try cache first: get → miss falls through → parse → set
	if s.cache != nil {
		cached, err := s.cache.Get(ctx, cacheKey)
		if err == nil && cached != "" {
			result, err := parser.DeserializeResponse(cached)
			if err == nil {
				result.Name = req.Name
				result.Phone = req.Phone
				result.Company = req.Company
				return &ParseResult{
					Response:    result,
					Method:      MethodCache,
					ParseTimeMs: int(time.Since(start).Milliseconds()),
				}, nil
			}
		}
	}

	if result, ok := s.ruleEngine.Parse(cleanedAddr); ok {
		result.Name = req.Name
		result.Phone = req.Phone
		result.Company = req.Company

		if s.cache != nil {
			data, _ := parser.SerializeResponse(result)
			_ = s.cache.Set(ctx, cacheKey, data, 24*time.Hour)
		}

		return &ParseResult{
			Response:   result,
			Method:     MethodRule,
			ParseTimeMs: int(time.Since(start).Milliseconds()),
		}, nil
	}

	llmResult, err := s.llmParser.Parse(ctx, cleanedAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse address: %w", err)
	}

	llmResult.Name = req.Name
	llmResult.Phone = req.Phone
	llmResult.Company = req.Company

	if s.cache != nil {
		data, _ := parser.SerializeResponse(llmResult)
		_ = s.cache.Set(ctx, cacheKey, data, 24*time.Hour)
	}

	return &ParseResult{
		Response:   llmResult,
		Method:     MethodLLM,
		ParseTimeMs: int(time.Since(start).Milliseconds()),
	}, nil
}
