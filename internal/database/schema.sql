-- First, add UUID extension if not exists
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- First, create the base tables that others depend on
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    balance DECIMAL(20,8) DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP,
    status VARCHAR(20),
    verification_level VARCHAR(20)
);

CREATE TABLE admin_users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP
);

-- Then create tables that reference the base tables
CREATE TABLE games (
    game_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    crash_point DECIMAL(10,2) NOT NULL,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    hash VARCHAR(64) NOT NULL
);

CREATE TABLE payment_methods (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    type VARCHAR(20) NOT NULL,
    address VARCHAR(255) NOT NULL,
    label VARCHAR(100),
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Then create tables that reference both base and dependent tables
CREATE TABLE bets (
    id SERIAL PRIMARY KEY,
    game_id UUID REFERENCES games(game_id),
    user_id UUID REFERENCES users(id),
    amount DECIMAL(20,8) NOT NULL,
    cashed_out BOOLEAN DEFAULT FALSE,
    cashout_multiplier DECIMAL(10,2),
    win_amount DECIMAL(20,8),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    type VARCHAR(10) NOT NULL,
    balance_after DECIMAL(20,8) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE withdrawals (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    status VARCHAR(20) NOT NULL,
    payment_method_id INTEGER REFERENCES payment_methods(id),
    tx_hash VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status_updated_by INTEGER REFERENCES admin_users(id),
    status_updated_at TIMESTAMP,
    rejection_reason TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE deposits (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    status VARCHAR(20) NOT NULL,
    payment_method_id INTEGER REFERENCES payment_methods(id),
    tx_hash VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE user_settings (
    user_id UUID PRIMARY KEY,
    theme VARCHAR(20) DEFAULT 'light',
    sound_enabled BOOLEAN DEFAULT true,
    email_notifications BOOLEAN DEFAULT true,
    auto_cashout_enabled BOOLEAN DEFAULT false,
    auto_cashout_value DECIMAL(10,2),
    language VARCHAR(10) DEFAULT 'en',
    timezone VARCHAR(50) DEFAULT 'UTC',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE user_notes (
    id SERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    note TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE admin_actions (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER REFERENCES admin_users(id),
    action_type VARCHAR(50) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_id UUID NOT NULL,
    details JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE notifications (
    id SERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    priority VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX idx_bets_user_id ON bets(user_id);
CREATE INDEX idx_bets_game_id ON bets(game_id);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_created_at ON transactions(created_at);
CREATE INDEX idx_withdrawals_status ON withdrawals(status);
CREATE INDEX idx_deposits_user_id ON deposits(user_id);
CREATE INDEX idx_payment_methods_user_id ON payment_methods(user_id);