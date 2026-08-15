CREATE TABLE session_summaries (
	session_id          VARCHAR(36) PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
	content             TEXT NOT NULL,
	through_message_id  VARCHAR(36) NOT NULL,
	created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_session_summaries_updated_at
AFTER UPDATE ON session_summaries
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
	UPDATE session_summaries
	SET updated_at = CURRENT_TIMESTAMP
	WHERE session_id = NEW.session_id;
END;
