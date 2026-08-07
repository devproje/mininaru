ALTER TABLE sessions ADD COLUMN origin VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN external_id VARCHAR(64) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX idx_sessions_external ON sessions(origin, external_id)
WHERE origin != '' AND external_id != '';
