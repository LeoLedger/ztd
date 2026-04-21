package model

import (
	"strings"
	"time"
)

// ParseRequest accepts either the new free-text format (text) or the legacy
// structured format (Name/Phone/Company/Address). The text field takes precedence.
type ParseRequest struct {
	// Text is a free-text address string containing any combination of
	// name, phone, company, and address in any order with optional missing fields.
	// Example: "张三 13812345678 智腾达科技 广东省深圳市南山区桃源街道88号"
	Text string `json:"text"`
	// Legacy structured fields — kept for backward compatibility.
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Company string `json:"company"`
	Address string `json:"address"`
}

// RawFields holds the extracted fields before preprocessing/normalization.
// OriginalText preserves the unmodified input text so LLM can see the full context
// (e.g. company names that appear after the address portion).
type RawFields struct {
	Name         string
	Phone        string
	Company      string
	Address      string
	OriginalText string // unprocessed original text for LLM context
}

// Text returns a space-joined concatenation of all non-empty fields.
func (r RawFields) Text() string {
	parts := []string{}
	if r.Name != "" {
		parts = append(parts, r.Name)
	}
	if r.Phone != "" {
		parts = append(parts, r.Phone)
	}
	if r.Company != "" {
		parts = append(parts, r.Company)
	}
	if r.Address != "" {
		parts = append(parts, r.Address)
	}
	return strings.Join(parts, " ")
}

type ParseResponse struct {
	Name     string `json:"name,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Company  string `json:"company,omitempty"`
	Province string `json:"province,omitempty"`
	City     string `json:"city,omitempty"`
	District string `json:"district,omitempty"`
	Street   string `json:"street,omitempty"`
	Detail   string `json:"detail,omitempty"`
	FullAddr string `json:"full_address,omitempty"`
}

type ParseHistory struct {
	ID             int64     `json:"id"`
	RequestID      string    `json:"request_id"`
	AppID          string    `json:"app_id"`
	InputHash      string    `json:"input_hash"`
	InputName      string    `json:"input_name"`
	InputPhone     string    `json:"input_phone"`
	InputCompany   string    `json:"input_company"`
	InputAddress   string    `json:"input_address"`
	OutputProvince string    `json:"output_province"`
	OutputCity     string    `json:"output_city"`
	OutputDistrict string    `json:"output_district"`
	OutputStreet   string    `json:"output_street"`
	OutputDetail   string    `json:"output_detail"`
	ParseMethod    string    `json:"parse_method"`
	ParseTimeMs    int       `json:"parse_time_ms"`
	CreatedAt      time.Time `json:"created_at"`
}
