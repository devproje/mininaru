CREATE TABLE tool_calls (
	id          VARCHAR(255) PRIMARY KEY,
	call_id     VARCHAR(255) NOT NULL,
	message_id  VARCHAR(36) NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
	name        VARCHAR(255) NOT NULL,
	arguments   TEXT NOT NULL,
	result      TEXT NOT NULL DEFAULT '',
	status      VARCHAR(16) NOT NULL,
	error       TEXT NOT NULL DEFAULT '',
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tool_calls_message_id ON tool_calls(message_id);
CREATE INDEX idx_tool_calls_call_id ON tool_calls(call_id);
