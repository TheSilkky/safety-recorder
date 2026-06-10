ALTER TABLE contact_public_keys ADD COLUMN replaced_at TEXT;
ALTER TABLE contact_public_keys ADD COLUMN lost_at TEXT;
ALTER TABLE contact_public_keys ADD COLUMN replaced_by_public_key_id TEXT REFERENCES contact_public_keys(id) ON DELETE SET NULL;
