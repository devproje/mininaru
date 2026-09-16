-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-or-later

CREATE TABLE IF NOT EXISTS session_summaries(
    session_id VARCHAR(36) PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    through_message_id VARCHAR(36) NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
