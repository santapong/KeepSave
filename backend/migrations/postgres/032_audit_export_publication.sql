ALTER TABLE audit_exports ADD COLUMN session_id UUID REFERENCES session_tokens(id);
ALTER TABLE audit_exports ADD COLUMN state TEXT NOT NULL DEFAULT 'ready' CHECK(state IN ('pending','ready','failed'));
