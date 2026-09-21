-- SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
-- SPDX-License-Identifier: GPL-3.0-only

ALTER TABLE agents ADD COLUMN selected BOOLEAN NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_selected
    ON agents(selected)
    WHERE selected = 1;

UPDATE agents SET selected = 1
WHERE id = (SELECT id FROM agents ORDER BY name ASC LIMIT 1);

DROP INDEX IF EXISTS idx_providers_active;

ALTER TABLE providers DROP COLUMN active;
