CREATE TABLE skill_uses (
	id          VARCHAR(36) PRIMARY KEY,
	skill       VARCHAR(64) NOT NULL,
	scope       VARCHAR(16) NOT NULL DEFAULT '',
	path        TEXT NOT NULL DEFAULT '',
	rel         TEXT NOT NULL DEFAULT '',
	session_id  VARCHAR(36) NOT NULL DEFAULT '',
	call_id     VARCHAR(255) NOT NULL DEFAULT '',
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_skill_uses_skill ON skill_uses(skill);
CREATE INDEX idx_skill_uses_session_id ON skill_uses(session_id);
