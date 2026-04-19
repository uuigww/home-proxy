CREATE TABLE users (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL UNIQUE,
    vless_uuid   TEXT,
    socks_user   TEXT,
    socks_pass   TEXT,
    limit_bytes  INTEGER NOT NULL DEFAULT 0,
    used_bytes   INTEGER NOT NULL DEFAULT 0,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);
CREATE INDEX idx_users_enabled ON users(enabled);

CREATE TABLE admin_prefs (
    tg_id                    INTEGER PRIMARY KEY,
    lang                     TEXT    NOT NULL DEFAULT 'ru',
    notify_critical          INTEGER NOT NULL DEFAULT 1,
    notify_important         INTEGER NOT NULL DEFAULT 1,
    notify_info              INTEGER NOT NULL DEFAULT 1,
    notify_info_others_only  INTEGER NOT NULL DEFAULT 0,
    notify_security          INTEGER NOT NULL DEFAULT 1,
    notify_daily             INTEGER NOT NULL DEFAULT 1,
    notify_nonadmin_spam     INTEGER NOT NULL DEFAULT 0,
    digest_hour              INTEGER NOT NULL DEFAULT 9,
    quiet_from_hour          INTEGER NOT NULL DEFAULT 23,
    quiet_to_hour            INTEGER NOT NULL DEFAULT 7,
    updated_at               DATETIME NOT NULL
);

CREATE TABLE sessions (
    tg_id        INTEGER PRIMARY KEY,
    chat_id      INTEGER NOT NULL,
    message_id   INTEGER NOT NULL,
    screen       TEXT    NOT NULL,
    wizard_json  TEXT    NOT NULL DEFAULT '{}',
    updated_at   DATETIME NOT NULL
);

CREATE TABLE usage_history (
    user_id        INTEGER NOT NULL,
    day            DATE    NOT NULL,
    uplink_bytes   INTEGER NOT NULL DEFAULT 0,
    downlink_bytes INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE reality_keys (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    private_key  TEXT NOT NULL,
    public_key   TEXT NOT NULL,
    short_id     TEXT NOT NULL,
    dest         TEXT NOT NULL,
    server_name  TEXT NOT NULL,
    created_at   DATETIME NOT NULL
);

CREATE TABLE warp_peer (
    id               INTEGER PRIMARY KEY CHECK (id = 1),
    private_key      TEXT NOT NULL,
    peer_public_key  TEXT NOT NULL,
    ipv4             TEXT NOT NULL,
    ipv6             TEXT NOT NULL,
    endpoint         TEXT NOT NULL,
    mtu              INTEGER NOT NULL DEFAULT 1280,
    updated_at       DATETIME NOT NULL
);
