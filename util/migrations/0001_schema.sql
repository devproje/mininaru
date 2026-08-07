CREATE TABLE sessions (
	id			VARCHAR(36) PRIMARY KEY,
	agent_id	VARCHAR(36) NOT NULL,
	name		VARCHAR(255) NOT NULL,
	created_at	DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at	DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_sessions_updated_at
AFTER UPDATE ON sessions
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
	UPDATE sessions
	SET updated_at = CURRENT_TIMESTAMP
	WHERE id = NEW.id;
END;

CREATE TABLE messages (
	id			VARCHAR(36) PRIMARY KEY,
	session_id	VARCHAR(36) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	role		VARCHAR(16) NOT NULL,
	content		TEXT NOT NULL,
	created_at	DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_session_id ON messages(session_id);
