ALTER TABLE auth_sessions
  ADD COLUMN second_factor_verified_at TEXT;

ALTER TABLE auth_sessions
  ADD COLUMN second_factor_factor_id TEXT;

ALTER TABLE auth_sessions
  ADD COLUMN second_factor_method TEXT CHECK (
    second_factor_method IS NULL
    OR second_factor_method IN ('totp')
  );

CREATE TABLE IF NOT EXISTS account_totp_second_factors (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  factor_state TEXT NOT NULL CHECK (
    factor_state IN ('pending', 'active')
  ),
  secret TEXT NOT NULL CHECK (secret <> ''),
  period_seconds INTEGER NOT NULL CHECK (period_seconds > 0),
  digits INTEGER NOT NULL CHECK (digits IN (6, 8)),
  algorithm TEXT NOT NULL CHECK (
    algorithm IN ('SHA1')
  ),
  last_used_time_step INTEGER,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  verified_at TEXT,
  UNIQUE(account_id)
);

CREATE INDEX IF NOT EXISTS idx_account_totp_second_factors_account_id
  ON account_totp_second_factors(account_id);
CREATE INDEX IF NOT EXISTS idx_account_totp_second_factors_account_state
  ON account_totp_second_factors(account_id, factor_state);
