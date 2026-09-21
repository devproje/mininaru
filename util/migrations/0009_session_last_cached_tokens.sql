-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-only

ALTER TABLE sessions ADD COLUMN last_cached_tokens INTEGER NOT NULL DEFAULT 0;
