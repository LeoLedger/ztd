package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/your-name/address-parse/config"
	"github.com/your-name/address-parse/internal/middleware"
	"github.com/your-name/address-parse/internal/model"
	"github.com/your-name/address-parse/internal/parser"
	"github.com/your-name/address-parse/internal/repository"
	"github.com/your-name/address-parse/internal/service"
	"github.com/your-name/address-parse/pkg/response"
)

type AddressHandler struct {
	parserService *service.ParserService
	historyRepo   repository.HistoryRepo
}

func NewAddressHandler(parserService *service.ParserService, historyRepo repository.HistoryRepo) *AddressHandler {
	return &AddressHandler{
		parserService: parserService,
		historyRepo:   historyRepo,
	}
}

func SetupRouter(h *AddressHandler, cfg *config.Config, redisClient *redis.Client) http.Handler {
	r := chi.NewRouter()

	// Health endpoint - no auth required
	r.Use(LogMiddleware)
	r.Get("/health", h.HealthCheck)

	// API routes - protected by signature + rate limit
	r.Group(func(api chi.Router) {
		api.Use(middleware.NewSignatureMiddleware(cfg, redisClient))
		api.Use(middleware.NewLimiterMiddleware(cfg, redisClient))
		api.Post("/api/v1/address/parse", h.ParseAddress)
	})

	return r
}

func (h *AddressHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response.SuccessWithMessage(w, "ok", map[string]string{"status": "healthy"})
}

func (h *AddressHandler) ParseAddress(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		response.BadRequest(w, "failed to read body")
		return
	}
	// Replace unescaped literal whitespace with a space so the JSON decoder sees
	// valid JSON even when the client sends raw newlines/tabs inside string values.
	cooked := replaceUnescapedNewlines(raw)

	var req model.ParseRequest
	if err := json.Unmarshal(cooked, &req); err != nil {
		response.BadRequest(w, "invalid JSON body")
		return
	}

	if req.Address == "" {
		response.BadRequest(w, "address is required")
		return
	}

	// Normalize input: trim whitespace and unify separator characters
	req.Name = parser.NormalizeText(req.Name)
	req.Phone = parser.NormalizeText(req.Phone)
	req.Company = parser.NormalizeText(req.Company)
	req.Address = parser.NormalizeText(req.Address)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := h.parserService.Parse(ctx, &req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeParseFailed, "address parsing failed: "+err.Error())
		return
	}

	requestID := uuid.New().String()
	appID := r.Header.Get("X-App-Id")
	inputHash := parser.HashAddress(req.Address)

	history := repository.BuildParseHistory(
		requestID, appID, inputHash, &req, result.Response, result.Method, result.ParseTimeMs,
	)

	if h.historyRepo != nil {
		// Use a detached goroutine with its own context and a hard timeout so it cannot
		// outlive the request. Panic inside the goroutine is recovered to prevent crashes.
		go func(hist *model.ParseHistory) {
			defer func() {
				if p := recover(); p != nil {
					// log or suppress — do not crash the service
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = h.historyRepo.Save(ctx, hist)
		}(history)
	}

	response.Success(w, result.Response)
}

func LogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = time.Since(start)
	})
}

// replaceUnescapedNewlines replaces literal \n, \r, and \t bytes with a space, both outside
// and inside JSON string values. This handles the case where a client sends raw
// whitespace inside a JSON string value, which is invalid per RFC 8259 but common
// in practice with poorly-behaved HTTP clients. After replacement the result is
// valid JSON (e.g. "广东省深圳\t南山区" → "广东省深圳 南山区").
func replaceUnescapedNewlines(data []byte) []byte {
	var result []byte
	inString := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if !inString && (b == '\n' || b == '\r' || b == '\t') {
			result = append(result, ' ')
			continue
		}
		if b == '"' {
			escaped := false
			count := 0
			for j := len(result) - 1; j >= 0 && result[j] == '\\'; j-- {
				count++
			}
			if count%2 == 1 {
				escaped = true
			}
			if !escaped {
				inString = !inString
			}
		} else if inString && (b == '\n' || b == '\r' || b == '\t') {
			result = append(result, ' ')
			continue
		}
		result = append(result, b)
	}
	return result
}
