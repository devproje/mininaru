-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-or-later

CREATE TABLE IF NOT EXISTS attachments(
    id VARCHAR(36) PRIMARY KEY,
    session_id VARCHAR(36) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    message_id VARCHAR(36) REFERENCES messages(id) ON DELETE SET NULL,
    mime VARCHAR(128) NOT NULL,
    bytes INTEGER NOT NULL DEFAULT 0,
    path TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_attachments_session_id ON attachments(session_id);
CREATE INDEX IF NOT EXISTS idx_attachments_message_id ON attachments(message_id);
