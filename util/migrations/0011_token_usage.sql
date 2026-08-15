CREATE TABLE token_usage (
	id                 VARCHAR(36) PRIMARY KEY,
	session_id         VARCHAR(36) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	message_id         VARCHAR(36) NOT NULL DEFAULT '',
	kind               VARCHAR(16) NOT NULL,
	prompt_tokens      INTEGER NOT NULL DEFAULT 0,
	completion_tokens  INTEGER NOT NULL DEFAULT 0,
	total_tokens       INTEGER NOT NULL DEFAULT 0,
	created_at         DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_token_usage_session_id ON token_usage(session_id);
