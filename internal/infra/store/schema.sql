-- Pager SQLite Schema v2
-- All tables use t_ prefix and id auto-increment primary key.

CREATE TABLE IF NOT EXISTS t_sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key      TEXT    NOT NULL UNIQUE,            -- 会话唯一标识 (session_id 或 host:cwd:tty)
    session_id       TEXT    NOT NULL DEFAULT '',        -- CC 原生 session_id
    agent            TEXT    NOT NULL DEFAULT 'claude-code', -- 代理类型: claude-code / codex / codebuddy
    host             TEXT    NOT NULL DEFAULT '',
    cwd              TEXT    NOT NULL DEFAULT '',
    project_name     TEXT    NOT NULL DEFAULT '',        -- CWD 末段，分组排序键
    tty              TEXT    NOT NULL DEFAULT '',
    term_program     TEXT    NOT NULL DEFAULT '',        -- iTerm2 / Apple_Terminal / WezTerm
    iterm_session_id TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'working', -- 当前状态: working / waiting / done / error
    agent_label      TEXT    NOT NULL DEFAULT 'CC',
    created_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')),
    updated_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')),
    deleted_at       TEXT             DEFAULT NULL       -- 软删除标记 (NULL=未删除)
);

CREATE INDEX IF NOT EXISTS idx_sessions_project ON t_sessions(project_name, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_updated ON t_sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_deleted ON t_sessions(deleted_at);

CREATE TABLE IF NOT EXISTS t_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_key     TEXT    NOT NULL,                   -- FK → t_sessions.session_key
    agent_label     TEXT    NOT NULL DEFAULT '',
    event_type      TEXT    NOT NULL,                   -- CC-native CamelCase: PreToolUse/PostToolUse/Stop/StopFailure/Notification/PermissionRequest/Elicitation/SessionStart/...
    tool_name       TEXT    NOT NULL DEFAULT '',
    tool_use_id     TEXT    NOT NULL DEFAULT '',
    content         TEXT    NOT NULL DEFAULT '',        -- 截断显示内容 (≤60 rune)
    content_raw     TEXT    NOT NULL DEFAULT '',        -- 完整内容摘要
    permission_mode TEXT    NOT NULL DEFAULT '',        -- bypassPermissions / default / etc.
    raw_payload     BLOB             DEFAULT NULL,      -- 完整 stdin JSON
    timestamp       TEXT    NOT NULL DEFAULT (datetime('now','localtime')),

    FOREIGN KEY (session_key) REFERENCES t_sessions(session_key)
);

CREATE INDEX IF NOT EXISTS idx_events_session_ts ON t_events(session_key, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_type       ON t_events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_timestamp  ON t_events(timestamp DESC);
