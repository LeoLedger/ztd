package model

import "time"

type ParseRequest struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Company string `json:"company"`
	Address string `json:"address" validate:"required"`
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
