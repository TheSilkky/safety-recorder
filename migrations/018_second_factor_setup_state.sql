ALTER TABLE accounts ADD COLUMN second_factor_setup_state TEXT NOT NULL DEFAULT 'not_required' CHECK (
  second_factor_setup_state IN (
    'not_required',
    'setup_required',
    'complete'
  )
);

CREATE INDEX IF NOT EXISTS idx_accounts_second_factor_setup_state
  ON accounts(second_factor_setup_state);
