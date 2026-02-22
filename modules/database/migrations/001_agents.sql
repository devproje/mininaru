-- SPDX-License-Identifier: GPL-2.0-or-later
-- Copyright (C) 2022-2026 Project_IO

-- with handle restrict references
PRAGMA foreign_keys = ON;

-- agent engine table
CREATE TABLE IF NOT EXISTS agent_engine(
	id VARCHAR(50) NOT NULL,
	api_endpoint VARCHAR(255) NOT NULL,
	api_key VARCHAR(255) DEFAULT NULL,
	model VARCHAR(100) NOT NULL,
	PRIMARY KEY(id)
);

-- agent data table
CREATE TABLE IF NOT EXISTS agents(
	id VARCHAR(50) NOT NULL,
	`name` VARCHAR(255) NOT NULL,
	engine VARCHAR(50) DEFAULT NULL,
	`default` TINYINT(1) DEFAULT 0 NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(id),
	FOREIGN KEY(engine) REFERENCES agent_engine(id)
		ON UPDATE CASCADE ON DELETE RESTRICT
);

-- agents update trigger
CREATE TRIGGER update_agents_updated_at UPDATE ON agents
FOR EACH ROW
	WHEN NEW.updated_at = OLD.updated_at
BEGIN
	UPDATE agents SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

-- agent session table
-- CREATE TABLE IF NOT EXISTS agent_session(
-- 	id VARCHAR(36) NOT NULL,
-- 	agent_id VARCHAR(50) NOT NULL,
-- 	`name` VARCHAR(255) NOT NULL,
-- 	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
-- 	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
-- 	PRIMARY KEY(agent_id, id),
-- 	FOREIGN KEY(agent_id) REFERENCES agents(id)
-- 		ON UPDATE CASCADE ON DELETE CASCADE
-- );

-- agent session context trigger
-- CREATE TRIGGER update_agent_session_updated_at UPDATE ON agent_session
-- FOR EACH ROW
-- WHEN NEW.updated_at = OLD.updated_at
-- BEGIN
-- 	UPDATE agent_session SET updated_at = CURRENT_TIMESTAMP	WHERE id = OLD.id;
-- END;
