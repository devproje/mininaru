-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-only

CREATE TABLE IF NOT EXISTS web_providers(
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    kind VARCHAR(20) NOT NULL,
    api_key VARCHAR(255) NOT NULL DEFAULT '',
    base_url VARCHAR(255) NOT NULL DEFAULT '',
    selected BOOLEAN NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_web_providers_selected
    ON web_providers(selected)
    WHERE selected = 1;
