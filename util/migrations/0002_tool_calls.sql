-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-or-later

CREATE TABLE IF NOT EXISTS tool_calls(
    id VARCHAR(36) PRIMARY KEY,
    message_id VARCHAR(36) NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    call_id VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    arguments TEXT NOT NULL,
    result TEXT NOT NULL DEFAULT '',
    status TEXT CHECK(status IN ('pending', 'completed', 'failed')) NOT NULL DEFAULT 'pending',
    error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tool_calls_message_id ON tool_calls(message_id);
