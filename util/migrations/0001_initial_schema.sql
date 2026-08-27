-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-or-later

CREATE TABLE IF NOT EXISTS providers(
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    api_key VARCHAR(255) NOT NULL DEFAULT '',
    base_url VARCHAR(255) NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_providers_active
    ON providers(active)
    WHERE active = 1;

CREATE TABLE IF NOT EXISTS agents(
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    model VARCHAR(50) NOT NULL,
    soul TEXT NOT NULL DEFAULT '',
    thinking_level TEXT CHECK(thinking_level IN ('off', 'low', 'medium', 'high', 'max')) DEFAULT 'medium',
    max_context INTEGER NOT NULL DEFAULT 24000
);

CREATE TABLE IF NOT EXISTS sessions(
    id VARCHAR(36) PRIMARY KEY,
    agent_id VARCHAR(36) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id);

CREATE TABLE IF NOT EXISTS messages(
    id VARCHAR(36) PRIMARY KEY,
    session_id VARCHAR(36) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    status TEXT CHECK(status IN ('pending', 'completed', 'failed', 'cancelled')) NOT NULL DEFAULT 'pending',
    error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);
