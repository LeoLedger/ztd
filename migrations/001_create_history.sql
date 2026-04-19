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
