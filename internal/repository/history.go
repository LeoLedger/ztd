package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-name/address-parse/internal/model"
)

type HistoryRepository struct {
	pool *pgxpool.Pool
}

func NewHistoryRepository(databaseURL string) (*HistoryRepository, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	poolConfig.MaxConns = 20
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &HistoryRepository{pool: pool}
	if err := repo.initSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return repo, nil
}

func (r *HistoryRepository) initSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS parse_history (
		id BIGSERIAL PRIMARY KEY,
		request_id VARCHAR(64) UNIQUE NOT NULL,
		app_id VARCHAR(64) NOT NULL,
		input_hash VARCHAR(64) NOT NULL,
		input_name VARCHAR(100),
		input_phone VARCHAR(20),
		input_company VARCHAR(200),
		input_address TEXT NOT NULL,
		output_province VARCHAR(50),
		output_city VARCHAR(50),
		output_district VARCHAR(50),
		output_street VARCHAR(100),
		output_detail TEXT,
		parse_method VARCHAR(20),
		parse_time_ms INT,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_history_app_id ON parse_history(app_id);
	CREATE INDEX IF NOT EXISTS idx_history_input_hash ON parse_history(input_hash);
	CREATE INDEX IF NOT EXISTS idx_history_created_at ON parse_history(created_at);
	`
	_, err := r.pool.Exec(ctx, schema)
	return err
}

func (r *HistoryRepository) Save(ctx context.Context, h *model.ParseHistory) error {
	query := `
	INSERT INTO parse_history (
		request_id, app_id, input_hash, input_name, input_phone, input_company,
		input_address, output_province, output_city, output_district, output_street,
		output_detail, parse_method, parse_time_ms
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	ON CONFLICT (request_id) DO NOTHING
	`
	_, err := r.pool.Exec(ctx, query,
		h.RequestID, h.AppID, h.InputHash, h.InputName, h.InputPhone, h.InputCompany,
		h.InputAddress, h.OutputProvince, h.OutputCity, h.OutputDistrict, h.OutputStreet,
		h.OutputDetail, h.ParseMethod, h.ParseTimeMs,
	)
	return err
}

func (r *HistoryRepository) FindByHash(ctx context.Context, inputHash string) (*model.ParseHistory, error) {
	query := `
	SELECT id, request_id, app_id, input_hash, input_name, input_phone, input_company,
		input_address, output_province, output_city, output_district, output_street,
		output_detail, parse_method, parse_time_ms, created_at
	FROM parse_history WHERE input_hash = $1
	ORDER BY created_at DESC LIMIT 1
	`
	var h model.ParseHistory
	err := r.pool.QueryRow(ctx, query, inputHash).Scan(
		&h.ID, &h.RequestID, &h.AppID, &h.InputHash, &h.InputName, &h.InputPhone, &h.InputCompany,
		&h.InputAddress, &h.OutputProvince, &h.OutputCity, &h.OutputDistrict, &h.OutputStreet,
		&h.OutputDetail, &h.ParseMethod, &h.ParseTimeMs, &h.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *HistoryRepository) List(ctx context.Context, appID string, limit, offset int) ([]*model.ParseHistory, error) {
	query := `
	SELECT id, request_id, app_id, input_hash, input_name, input_phone, input_company,
		input_address, output_province, output_city, output_district, output_street,
		output_detail, parse_method, parse_time_ms, created_at
	FROM parse_history WHERE app_id = $1
	ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, appID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []*model.ParseHistory
	for rows.Next() {
		var h model.ParseHistory
		err := rows.Scan(
			&h.ID, &h.RequestID, &h.AppID, &h.InputHash, &h.InputName, &h.InputPhone, &h.InputCompany,
			&h.InputAddress, &h.OutputProvince, &h.OutputCity, &h.OutputDistrict, &h.OutputStreet,
			&h.OutputDetail, &h.ParseMethod, &h.ParseTimeMs, &h.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		histories = append(histories, &h)
	}
	return histories, nil
}

func (r *HistoryRepository) Close() error {
	r.pool.Close()
	return nil
}

func NewInMemoryHistoryRepository() *InMemoryHistoryRepository {
	return &InMemoryHistoryRepository{
		records: make([]*model.ParseHistory, 0),
	}
}

type InMemoryHistoryRepository struct {
	mu      sync.RWMutex
	records []*model.ParseHistory
}

func (r *InMemoryHistoryRepository) Save(ctx context.Context, h *model.ParseHistory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, h)
	return nil
}

func (r *InMemoryHistoryRepository) FindByHash(ctx context.Context, inputHash string) (*model.ParseHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for i := len(r.records) - 1; i >= 0; i-- {
		if r.records[i].InputHash == inputHash {
			return r.records[i], nil
		}
	}
	return nil, nil
}

func (r *InMemoryHistoryRepository) List(ctx context.Context, appID string, limit, offset int) ([]*model.ParseHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*model.ParseHistory
	for _, h := range r.records {
		if h.AppID == appID {
			result = append(result, h)
		}
	}
	if offset >= len(result) {
		return []*model.ParseHistory{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (r *InMemoryHistoryRepository) Close() error {
	return nil
}

type HistoryRepo interface {
	Save(ctx context.Context, h *model.ParseHistory) error
	FindByHash(ctx context.Context, inputHash string) (*model.ParseHistory, error)
	List(ctx context.Context, appID string, limit, offset int) ([]*model.ParseHistory, error)
	Close() error
}

func BuildParseHistory(requestID, appID, inputHash string, req *model.ParseRequest, result *model.ParseResponse, method string, parseTimeMs int) *model.ParseHistory {
	return &model.ParseHistory{
		RequestID:      requestID,
		AppID:          appID,
		InputHash:      inputHash,
		InputName:      req.Name,
		InputPhone:     req.Phone,
		InputCompany:   req.Company,
		InputAddress:   req.Address,
		OutputProvince: result.Province,
		OutputCity:     result.City,
		OutputDistrict: result.District,
		OutputStreet:   result.Street,
		OutputDetail:   result.Detail,
		ParseMethod:    method,
		ParseTimeMs:    parseTimeMs,
		CreatedAt:      time.Now(),
	}
}
