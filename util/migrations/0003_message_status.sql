ALTER TABLE messages ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'completed';
ALTER TABLE messages ADD COLUMN error TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_messages_session_status ON messages(session_id, status);
