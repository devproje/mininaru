CREATE TABLE memories (
	id          VARCHAR(36) PRIMARY KEY,
	content     TEXT NOT NULL UNIQUE,
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_memories_updated_at
AFTER UPDATE ON memories
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
	UPDATE memories
	SET updated_at = CURRENT_TIMESTAMP
	WHERE id = NEW.id;
END;
