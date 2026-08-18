CREATE TABLE rpc_clients (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    public_key BLOB NOT NULL,
    certificate_serial TEXT NOT NULL UNIQUE,
    paired_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL DEFAULT 0,
    revoked_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE rpc_pairings (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    public_key BLOB NOT NULL,
    certificate_pem BLOB NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    status TEXT NOT NULL
);

CREATE INDEX rpc_pairings_expires_at ON rpc_pairings(expires_at);
