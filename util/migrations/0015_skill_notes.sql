CREATE TABLE skill_notes (
	id          VARCHAR(36) PRIMARY KEY,
	skill       VARCHAR(64) NOT NULL,
	note        TEXT NOT NULL,
	session_id  VARCHAR(36) NOT NULL DEFAULT '',
	applied_at  DATETIME,
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_skill_notes_skill ON skill_notes(skill);
CREATE INDEX idx_skill_notes_pending ON skill_notes(skill, applied_at);

CREATE TABLE skill_revisions (
	id          VARCHAR(36) PRIMARY KEY,
	skill       VARCHAR(64) NOT NULL,
	scope       VARCHAR(16) NOT NULL DEFAULT '',
	path        TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	body        TEXT NOT NULL,
	reason      TEXT NOT NULL DEFAULT '',
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_skill_revisions_skill ON skill_revisions(skill, created_at);
