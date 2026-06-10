CREATE TABLE IF NOT EXISTS account_second_factors (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  factor_type TEXT NOT NULL CHECK (
    factor_type IN ('email_challenge')
  ),
  email_normalized TEXT NOT NULL CHECK (email_normalized <> ''),
  factor_state TEXT NOT NULL CHECK (
    factor_state IN ('pending', 'active')
  ),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  verified_at TEXT,
  UNIQUE(account_id, factor_type)
);

CREATE TABLE IF NOT EXISTS account_second_factor_challenges (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  factor_id TEXT NOT NULL REFERENCES account_second_factors(id) ON DELETE CASCADE,
  challenge_type TEXT NOT NULL CHECK (
    challenge_type IN ('email_setup')
  ),
  token_hash TEXT NOT NULL UNIQUE CHECK (
    length(token_hash) = 64
    AND token_hash NOT GLOB '*[^0-9a-f]*'
  ),
  email_normalized TEXT NOT NULL CHECK (email_normalized <> ''),
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_account_second_factors_account_id
  ON account_second_factors(account_id);
CREATE INDEX IF NOT EXISTS idx_account_second_factors_account_type
  ON account_second_factors(account_id, factor_type);
CREATE INDEX IF NOT EXISTS idx_account_second_factor_challenges_account_id
  ON account_second_factor_challenges(account_id);
CREATE INDEX IF NOT EXISTS idx_account_second_factor_challenges_factor_id
  ON account_second_factor_challenges(factor_id);
CREATE INDEX IF NOT EXISTS idx_account_second_factor_challenges_token_hash
  ON account_second_factor_challenges(token_hash);
CREATE INDEX IF NOT EXISTS idx_account_second_factor_challenges_expires_at
  ON account_second_factor_challenges(expires_at);
