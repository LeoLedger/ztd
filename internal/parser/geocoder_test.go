package parser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/your-name/address-parse/config"
)

func TestAMapGeocoder_New(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *config.GeocoderConfig
		wantNil bool
	}{
		{"nil config returns nil", nil, true},
		{"empty API key returns nil", &config.GeocoderConfig{APIKey: ""}, true},
		{"valid config returns geocoder", &config.GeocoderConfig{APIKey: "test-key", BaseURL: "https://restapi.amap.com/v3"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAMapGeocoder(tt.cfg)
			if tt.wantNil && got != nil {
				t.Errorf("want nil geocoder, got %v", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("want non-nil geocoder, got nil")
			}
		})
	}
}

func TestAMapGeocoder_Geocode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("address") == "" {
			t.Error("address param missing")
		}
		w.Write([]byte(`{
			"status": "1",
			"info": "OK",
			"count": 1,
			"geocodes": [{
				"province": "广东省",
				"city": "惠州市",
				"district": "惠城区",
				"township": "河南岸街道"
			}]
		}`))
	}))
	defer server.Close()

	g := NewAMapGeocoder(&config.GeocoderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result := g.Geocode(context.Background(), "广东省惠州市河南岸街道金湖社区张屋山一巷二号")
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.District != "惠城区" {
		t.Errorf("District = %q, want %q", result.District, "惠城区")
	}
	if result.City != "惠州市" {
		t.Errorf("City = %q, want %q", result.City, "惠州市")
	}
	if result.Province != "广东省" {
		t.Errorf("Province = %q, want %q", result.Province, "广东省")
	}
	if result.Source != "amap" {
		t.Errorf("Source = %q, want %q", result.Source, "amap")
	}
}

func TestAMapGeocoder_Geocode_EmptyAddress(t *testing.T) {
	g := NewAMapGeocoder(&config.GeocoderConfig{APIKey: "test-key"})
	if got := g.Geocode(context.Background(), ""); got != nil {
		t.Errorf("Geocode('') = %v, want nil", got)
	}
}

func TestAMapGeocoder_Geocode_NilGeocoder(t *testing.T) {
	var g *AMapGeocoder
	if got := g.Geocode(context.Background(), "广东省惠州市惠城区"); got != nil {
		t.Errorf("nil.Geocode() = %v, want nil", got)
	}
}

func TestAMapGeocoder_Geocode_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status": "1", "info": "OK", "count": 0, "geocodes": []}`))
	}))
	defer server.Close()

	g := NewAMapGeocoder(&config.GeocoderConfig{APIKey: "test-key", BaseURL: server.URL})
	if got := g.Geocode(context.Background(), "完全不存在的地址xyz123"); got != nil {
		t.Errorf("Geocode(unknown) = %v, want nil", got)
	}
}

func TestAMapGeocoder_Geocode_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	g := NewAMapGeocoder(&config.GeocoderConfig{APIKey: "test-key", BaseURL: server.URL})
	if got := g.Geocode(context.Background(), "广东省惠州市"); got != nil {
		t.Errorf("Geocode() after API error = %v, want nil", got)
	}
}

func TestAMapGeocoder_Geocode_DirectCity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"status": "1",
			"info": "OK",
			"count": 1,
			"geocodes": [{
				"province": "北京市",
				"city": "北京市",
				"district": "海淀区",
				"township": "中关村街道"
			}]
		}`))
	}))
	defer server.Close()

	g := NewAMapGeocoder(&config.GeocoderConfig{APIKey: "test-key", BaseURL: server.URL})
	result := g.Geocode(context.Background(), "北京市海淀区中关村")
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.District != "海淀区" {
		t.Errorf("District = %q, want %q", result.District, "海淀区")
	}
	// For direct cities, province and city should be normalised.
	if result.Province != "北京市" {
		t.Errorf("Province = %q, want %q", result.Province, "北京市")
	}
	if result.City != "北京市" {
		t.Errorf("City = %q, want %q", result.City, "北京市")
	}
}

func TestAMapGeocoder_Geocode_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte(`{
			"status": "1",
			"info": "OK",
			"count": 1,
			"geocodes": [{
				"province": "广东省",
				"city": "惠州市",
				"district": "惠城区",
				"township": ""
			}]
		}`))
	}))
	defer server.Close()

	g := NewAMapGeocoder(&config.GeocoderConfig{APIKey: "test-key", BaseURL: server.URL})

	addr := "广东省惠州市惠城区"
	_ = g.Geocode(context.Background(), addr)
	_ = g.Geocode(context.Background(), addr)
	_ = g.Geocode(context.Background(), addr)

	if callCount != 1 {
		t.Errorf("API called %d times, want 1 (cache should prevent re-fetch)", callCount)
	}
}

func TestAMapGeocoder_Geocode_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response — context should cancel before we respond.
		select {}
	}))
	defer server.Close()

	g := NewAMapGeocoder(&config.GeocoderConfig{APIKey: "test-key", BaseURL: server.URL})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	if got := g.Geocode(ctx, "广东省惠州市"); got != nil {
		t.Errorf("Geocode(cancelled ctx) = %v, want nil", got)
	}
}

func TestIsDirectCity(t *testing.T) {
	cities := map[string]bool{
		"北京市": true,
		"上海市": true,
		"天津市": true,
		"重庆市": true,
		"深圳市": false,
		"广州市": false,
		"杭州市": false,
	}
	for city, want := range cities {
		got := isDirectCity(city)
		if got != want {
			t.Errorf("isDirectCity(%q) = %v, want %v", city, got, want)
		}
	}
}

func TestEnsureSuffix(t *testing.T) {
	tests := []struct {
		s      string
		suffix string
		want   string
	}{
		{"深圳", "市", "深圳市"},
		{"深圳市", "市", "深圳市"},
		{"", "市", ""},
		{"惠城区", "区", "惠城区"},
		{"惠城", "区", "惠城区"},
	}
	for _, tt := range tests {
		got := ensureSuffix(tt.s, tt.suffix)
		if got != tt.want {
			t.Errorf("ensureSuffix(%q, %q) = %q, want %q", tt.s, tt.suffix, got, tt.want)
		}
	}
}

func TestStripSuffixChars(t *testing.T) {
	tests := []struct {
		s      string
		suffix string
		want   string
	}{
		{"北京市", "市", "北京"},
		{"广东省", "省", "广东"},
		{"深圳市", "市", "深圳"},
		{"深圳", "市", "深圳"},
	}
	for _, tt := range tests {
		got := stripSuffixChars(tt.s, tt.suffix)
		if got != tt.want {
			t.Errorf("stripSuffixChars(%q, %q) = %q, want %q", tt.s, tt.suffix, got, tt.want)
		}
	}
}

func TestDistrictValidator_AutoFill_UsesGeocoder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Write([]byte(`{
			"status": "1",
			"info": "OK",
			"count": 1,
			"geocodes": [{
				"province": "广东省",
				"city": "惠州市",
				"district": "惠城区",
				"township": "河南岸街道"
			}]
		}`))
	}))
	defer server.Close()

	geocoder := NewAMapGeocoder(&config.GeocoderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	v := NewDistrictValidatorWithGeocoder(geocoder)

	result := v.AutoFillDistrictWithOriginal(
		context.Background(),
		"惠州市", "", "", "",
		"广东省惠州市辰芊科技有限公司张屋山一巷二号",
	)

	if result == nil {
		t.Fatal("expected auto-fill from geocoder, got nil")
	}
	if result.InferredDistrict != "惠城区" {
		t.Errorf("InferredDistrict = %q, want %q", result.InferredDistrict, "惠城区")
	}
	if result.InferenceSource != "geocoder" {
		t.Errorf("InferenceSource = %q, want %q", result.InferenceSource, "geocoder")
	}
}

func TestDistrictValidator_AutoFill_StreetLookupBeatsGeocoder(t *testing.T) {
	serverCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.Write([]byte(`{"status": "1", "count": 0, "geocodes": []}`))
	}))
	defer server.Close()

	geocoder := NewAMapGeocoder(&config.GeocoderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	v := NewDistrictValidatorWithGeocoder(geocoder)

	result := v.AutoFillDistrictWithOriginal(
		context.Background(),
		"惠州市", "", "河南岸街道", "金湖社区张屋山一巷二号",
		"广东省惠州市辰芊科技有限公司河南岸街道金湖社区张屋山一巷二号",
	)

	if result == nil {
		t.Fatal("expected street lookup result, got nil")
	}
	if result.InferredDistrict != "惠城区" {
		t.Errorf("InferredDistrict = %q, want %q", result.InferredDistrict, "惠城区")
	}
	if result.InferenceSource == "geocoder" {
		t.Errorf("InferenceSource = %q, geocoder should not have been called", result.InferenceSource)
	}
	if serverCalled {
		t.Error("geocoder was called but should not have been (street lookup should win)")
	}
}
