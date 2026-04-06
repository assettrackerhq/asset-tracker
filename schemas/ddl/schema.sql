CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_unique ON users (username);

CREATE TABLE IF NOT EXISTS assets (
    id VARCHAR(50) NOT NULL,
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    PRIMARY KEY (id, user_id)
);

CREATE TABLE IF NOT EXISTS asset_value_points (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id VARCHAR(50) NOT NULL,
    user_id UUID NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT now(),
    value NUMERIC(15,2) NOT NULL,
    currency VARCHAR(3) NOT NULL
);
