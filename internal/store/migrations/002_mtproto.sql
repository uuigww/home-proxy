ALTER TABLE users ADD COLUMN mtproto_enabled INTEGER NOT NULL DEFAULT 0;

CREATE TABLE mtg_config (
    id             INTEGER PRIMARY KEY CHECK (id = 1),
    secret         TEXT    NOT NULL,
    port           INTEGER NOT NULL,
    fake_tls_host  TEXT    NOT NULL,
    updated_at     DATETIME NOT NULL
);
