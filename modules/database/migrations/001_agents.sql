-- with handle restrict references
PRAGMA foreign_keys = ON;

-- agent engine table
CREATE TABLE IF NOT EXISTS agent_engine(
	`name` VARCHAR(50) NOT NULL,
	api_endpoint VARCHAR(255) NOT NULL,
	api_key VARCHAR(255) DEFAULT NULL,
	model VARCHAR(100) NOT NULL,
	PRIMARY KEY(`name`)
);

-- agent data table
CREATE TABLE IF NOT EXISTS agents(
	id VARCHAR(50) NOT NULL,
	`name` VARCHAR(255) NOT NULL,
	engine VARCHAR(50) DEFAULT NULL,
	`default` TINYINT(1) DEFAULT 0 NOT NULL,
	PRIMARY KEY(id),
	FOREIGN KEY(engine) REFERENCES agent_engine(`name`)
		ON UPDATE CASCADE ON DELETE RESTRICT
);

-- agent instructions file table
CREATE TABLE IF NOT EXISTS agent_instructions(
	agent_id VARCHAR(50) NOT NULL,
	`filename` VARCHAR(255),
	content TEXT DEFAULT NULL,
	PRIMARY KEY(agent_id, `filename`),
	FOREIGN KEY(agent_id) REFERENCES agents(id)
		ON UPDATE CASCADE ON DELETE CASCADE
);
