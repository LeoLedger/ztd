package parser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/your-name/address-parse/config"
)

// AMapGeocoder calls the 高德地图 (AMap) geocoding API to resolve
// the district (区/县) for an address. It is used as a last-resort
// fallback when both the LLM and the rule-based street→district lookup
// fail to determine the district.
//
// AMap returns the full administrative hierarchy (province → city →
// district → township) so it correctly handles cases where the input
// address omits the district level entirely.
//
// Results are cached in-memory per address hash for the lifetime of the
// process. Configure a Redis-based cache in front if multi-instance
// deployment is needed.
type AMapGeocoder struct {
	apiKey  string
	baseURL string
	client  *http.Client
	cache   map[string]*GeocodeResult
	mu      sync.RWMutex
}

type GeocodeResult struct {
	Province string
	City     string
	District string
	Source   string // "amap" or "cached"
}

type amapResponse struct {
	Status  string       `json:"status"`
	Info    string       `json:"info"`
	Count   int          `json:"count"`
	Geocodes []amapGeocode `json:"geocodes"`
}

type amapGeocode struct {
	Province string `json:"province"` // 直辖市时即为市名，如"北京市"
	City     string `json:"city"`     // 若城市为直辖市则返回直辖市名称
	District string `json:"district"`  // 区/县名称，如"海淀区"
	Township string `json:"township"`  // 街道/镇/乡名称
}

// NewAMapGeocoder creates a geocoder backed by the 高德地图 geocoding API.
func NewAMapGeocoder(cfg *config.GeocoderConfig) *AMapGeocoder {
	if cfg == nil || cfg.APIKey == "" {
		return nil
	}
	return &AMapGeocoder{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		client: &http.Client{Timeout: 5 * time.Second},
		cache:  make(map[string]*GeocodeResult),
	}
}

// Geocode resolves the district for the given address string.
// Returns nil if the API is not configured, the address is empty, or the
// API returns no results. Errors are logged but not returned so that a
// geocoding failure never propagates to the caller.
func (g *AMapGeocoder) Geocode(ctx context.Context, address string) *GeocodeResult {
	if g == nil || address == "" {
		return nil
	}

	// Check in-memory cache.
	cacheKey := g.cacheKey(address)
	g.mu.RLock()
	if cached, ok := g.cache[cacheKey]; ok {
		g.mu.RUnlock()
		return cached
	}
	g.mu.RUnlock()

	result := g.doGeocode(ctx, address)

	g.mu.Lock()
	g.cache[cacheKey] = result
	g.mu.Unlock()

	return result
}

// doGeocode performs the actual AMap API call.
func (g *AMapGeocoder) doGeocode(ctx context.Context, address string) *GeocodeResult {
	params := url.Values{}
	params.Set("address", address)
	params.Set("key", g.apiKey)
	params.Set("output", "json")
	params.Set("city", "全国") // 全国范围搜索，避免城市限制导致漏检

	req, err := http.NewRequestWithContext(ctx, "GET",
		g.baseURL+"/geocode/geo?"+params.Encode(), nil)
	if err != nil {
		return nil
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var ar amapResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil
	}

	if ar.Status != "1" || ar.Count == 0 || len(ar.Geocodes) == 0 {
		return nil
	}

	gc := ar.Geocodes[0]

	// AMap returns province/city name without suffix for direct-administered
	// municipalities (北京/上海/天津/重庆). Normalise them to the standard form.
	province := gc.Province
	city := gc.City
	district := gc.District

	// For direct municipalities, province and city are the same string
	// (e.g. "北京市" for both). Normalise both to standard form with "市" suffix.
	if isDirectCity(province) {
		// Strip "市" then re-add it to ensure consistent "北京市" form.
		province = ensureSuffix(stripSuffixChars(province, "市"), "市")
		city = province // direct cities: province == city
	} else {
		// Normal provinces: ensure "省" and "市" suffixes are present.
		province = ensureSuffix(province, "省")
		city = ensureSuffix(city, "市")
	}

	// District may be empty (e.g. "深圳市" without a district) — return nil.
	if district == "" {
		return nil
	}
	district = ensureSuffix(district, "区")

	return &GeocodeResult{
		Province: province,
		City:     city,
		District: district,
		Source:   "amap",
	}
}

// cacheKey returns a SHA-256 hash of the address string as the cache key.
func (g *AMapGeocoder) cacheKey(address string) string {
	h := sha256.Sum256([]byte(address))
	return hex.EncodeToString(h[:])
}

// isDirectCity returns true for the four direct-administered municipalities.
func isDirectCity(name string) bool {
	return strings.HasPrefix(name, "北京") ||
		strings.HasPrefix(name, "上海") ||
		strings.HasPrefix(name, "天津") ||
		strings.HasPrefix(name, "重庆")
}

// ensureSuffix adds suffix if not already present.
func ensureSuffix(s, suffix string) string {
	if s == "" {
		return ""
	}
	if !strings.HasSuffix(s, suffix) {
		return s + suffix
	}
	return s
}

// stripSuffixChars removes trailing "市" or "省" from the end.
func stripSuffixChars(s, suffix string) string {
	return strings.TrimSuffix(s, suffix)
}
