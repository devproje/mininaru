-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-or-later

ALTER TABLE sessions ADD COLUMN cwd VARCHAR(1024) NOT NULL DEFAULT '';
