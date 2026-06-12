ALTER TABLE contact_public_keys ADD COLUMN recipient_account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_contact_public_keys_recipient ON contact_public_keys(recipient_account_id);
