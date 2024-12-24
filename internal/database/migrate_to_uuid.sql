-- First, add UUID extension if not exists
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Temporarily disable foreign key constraints
SET session_replication_role = 'replica';

-- Drop existing constraints first
ALTER TABLE IF EXISTS bets DROP CONSTRAINT IF EXISTS bets_game_id_fkey;
ALTER TABLE IF EXISTS bets DROP CONSTRAINT IF EXISTS bets_user_id_fkey;
ALTER TABLE IF EXISTS payment_methods DROP CONSTRAINT IF EXISTS payment_methods_user_id_fkey;
ALTER TABLE IF EXISTS transactions DROP CONSTRAINT IF EXISTS transactions_user_id_fkey;
ALTER TABLE IF EXISTS withdrawals DROP CONSTRAINT IF EXISTS withdrawals_user_id_fkey;
ALTER TABLE IF EXISTS deposits DROP CONSTRAINT IF EXISTS deposits_user_id_fkey;
ALTER TABLE IF EXISTS user_settings DROP CONSTRAINT IF EXISTS user_settings_user_id_fkey;
ALTER TABLE IF EXISTS user_notes DROP CONSTRAINT IF EXISTS user_notes_user_id_fkey;

-- Add temporary columns to existing tables
ALTER TABLE users ADD COLUMN IF NOT EXISTS uuid_id UUID DEFAULT uuid_generate_v4();
ALTER TABLE games ADD COLUMN IF NOT EXISTS uuid_game_id UUID DEFAULT uuid_generate_v4();
ALTER TABLE bets ADD COLUMN IF NOT EXISTS temp_game_id UUID;
ALTER TABLE bets ADD COLUMN IF NOT EXISTS temp_user_id UUID;

-- Copy existing IDs to UUID columns, converting bigint to text first
UPDATE users SET uuid_id = CAST(CAST(id AS text) AS uuid) WHERE uuid_id IS NULL;
UPDATE games SET uuid_game_id = uuid_generate_v4() WHERE uuid_game_id IS NULL;

-- Store the mapping for games
CREATE TEMPORARY TABLE game_id_mapping AS
SELECT game_id, uuid_game_id FROM games;

-- Update foreign key references using the mapping
UPDATE bets b
SET temp_game_id = g.uuid_game_id
FROM game_id_mapping g
WHERE b.game_id = g.game_id AND b.temp_game_id IS NULL;

UPDATE bets b
SET temp_user_id = u.uuid_id
FROM users u
WHERE b.user_id = u.id::text AND b.temp_user_id IS NULL;

-- Drop old columns and rename new ones
ALTER TABLE users DROP COLUMN IF EXISTS id CASCADE;
ALTER TABLE users RENAME COLUMN uuid_id TO id;
ALTER TABLE users ADD PRIMARY KEY (id);

ALTER TABLE games DROP COLUMN IF EXISTS game_id CASCADE;
ALTER TABLE games RENAME COLUMN uuid_game_id TO game_id;
ALTER TABLE games ADD PRIMARY KEY (game_id);

ALTER TABLE bets DROP COLUMN IF EXISTS game_id;
ALTER TABLE bets DROP COLUMN IF EXISTS user_id;
ALTER TABLE bets RENAME COLUMN temp_game_id TO game_id;
ALTER TABLE bets RENAME COLUMN temp_user_id TO user_id;

-- Update other tables to use UUID
ALTER TABLE payment_methods ALTER COLUMN user_id TYPE UUID USING CAST(CAST(user_id AS text) AS uuid);
ALTER TABLE transactions ALTER COLUMN user_id TYPE UUID USING CAST(CAST(user_id AS text) AS uuid);
ALTER TABLE withdrawals ALTER COLUMN user_id TYPE UUID USING CAST(CAST(user_id AS text) AS uuid);
ALTER TABLE deposits ALTER COLUMN user_id TYPE UUID USING CAST(CAST(user_id AS text) AS uuid);
ALTER TABLE user_settings ALTER COLUMN user_id TYPE UUID USING CAST(CAST(user_id AS text) AS uuid);
ALTER TABLE user_notes ALTER COLUMN user_id TYPE UUID USING CAST(CAST(user_id AS text) AS uuid);
ALTER TABLE admin_actions ALTER COLUMN target_id TYPE UUID USING CAST(CAST(target_id AS text) AS uuid);

-- Recreate foreign key constraints
ALTER TABLE bets ADD CONSTRAINT bets_game_id_fkey FOREIGN KEY (game_id) REFERENCES games(game_id);
ALTER TABLE bets ADD CONSTRAINT bets_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE payment_methods ADD CONSTRAINT payment_methods_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE transactions ADD CONSTRAINT transactions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE withdrawals ADD CONSTRAINT withdrawals_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE deposits ADD CONSTRAINT deposits_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE user_settings ADD CONSTRAINT user_settings_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE user_notes ADD CONSTRAINT user_notes_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);

-- Drop temporary table
DROP TABLE IF EXISTS game_id_mapping;

-- Re-enable foreign key constraints
SET session_replication_role = 'origin';

-- Recreate indexes
DROP INDEX IF EXISTS idx_bets_user_id;
DROP INDEX IF EXISTS idx_bets_game_id;
DROP INDEX IF EXISTS idx_transactions_user_id;
DROP INDEX IF EXISTS idx_deposits_user_id;
DROP INDEX IF EXISTS idx_payment_methods_user_id;

CREATE INDEX idx_bets_user_id ON bets(user_id);
CREATE INDEX idx_bets_game_id ON bets(game_id);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_deposits_user_id ON deposits(user_id);
CREATE INDEX idx_payment_methods_user_id ON payment_methods(user_id); 