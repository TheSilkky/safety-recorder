ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS second_factor_setup_state TEXT NOT NULL DEFAULT 'not_required';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'accounts_second_factor_setup_state_check'
      AND conrelid = 'accounts'::regclass
  ) THEN
    ALTER TABLE accounts
      ADD CONSTRAINT accounts_second_factor_setup_state_check
      CHECK (
        second_factor_setup_state IN (
          'not_required',
          'setup_required',
          'complete'
        )
      );
  END IF;
END;
$$;

CREATE INDEX IF NOT EXISTS idx_accounts_second_factor_setup_state
  ON accounts(second_factor_setup_state);
