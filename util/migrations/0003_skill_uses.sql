-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-or-later

CREATE TABLE IF NOT EXISTS skill_uses(
    id VARCHAR(36) PRIMARY KEY,
    skill VARCHAR(64) NOT NULL,
    scope VARCHAR(16) NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    rel TEXT NOT NULL DEFAULT '',
    session_id VARCHAR(36) NOT NULL,
    call_id VARCHAR(100) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_skill_uses_session_id ON skill_uses(session_id);
