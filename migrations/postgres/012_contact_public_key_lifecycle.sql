ALTER TABLE contact_public_keys ADD COLUMN IF NOT EXISTS replaced_at TIMESTAMPTZ;
ALTER TABLE contact_public_keys ADD COLUMN IF NOT EXISTS lost_at TIMESTAMPTZ;
ALTER TABLE contact_public_keys ADD COLUMN IF NOT EXISTS replaced_by_public_key_id TEXT REFERENCES contact_public_keys(id) ON DELETE SET NULL;
