CREATE TABLE discord_users (
	bot_id      VARCHAR(36) NOT NULL,
	user_id     VARCHAR(64) NOT NULL,
	role        VARCHAR(16) NOT NULL DEFAULT 'user',
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (bot_id, user_id)
);

CREATE TABLE discord_pairings (
	code        VARCHAR(16) PRIMARY KEY,
	bot_id      VARCHAR(36) NOT NULL,
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_discord_pairings_bot_id ON discord_pairings(bot_id);
