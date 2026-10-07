-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-only

CREATE TABLE IF NOT EXISTS providers (
    id VARCHAR(36) PRIMARY KEY,
    "name" VARCHAR(63) UNIQUE NOT NULL,
    api_key VARCHAR(255) NOT NULL DEFAULT '',
    base_url VARCHAR(255) NOT NULL DEFAULT '' 
);
