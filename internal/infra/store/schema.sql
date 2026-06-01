-- Pager SQLite Schema
-- All tables use t_ prefix and id auto-increment primary key.

CREATE TABLE IF NOT EXISTS t_sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,  -- 自增主键
    session_key      TEXT    NOT NULL UNIQUE,            -- 会话唯一标识 (session_id 或 host:cwd:tty)
    session_id       TEXT    NOT NULL DEFAULT '',        -- CC 原生 session_id
    agent            TEXT    NOT NULL DEFAULT 'claude-code', -- 代理类型: claude-code / codex
    host             TEXT    NOT NULL DEFAULT '',        -- 主机名
    cwd              TEXT    NOT NULL DEFAULT '',        -- 工作目录
    project_name     TEXT    NOT NULL DEFAULT '',        -- 项目名 (CWD 末段，用于分组排序)
    tty              TEXT    NOT NULL DEFAULT '',        -- 终端 TTY 路径
    term_program     TEXT    NOT NULL DEFAULT '',        -- 终端程序: iTerm2 / Terminal.app
    iterm_session_id TEXT    NOT NULL DEFAULT '',        -- iTerm2 会话ID (跳转用)
    status           TEXT    NOT NULL DEFAULT 'active',  -- 当前状态: waiting/active/finished/error
    attention_level  TEXT    NOT NULL DEFAULT 'running', -- 注意力级别: attention/running/done
    agent_label      TEXT    NOT NULL DEFAULT 'CC',      -- 代理标签
    created_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')), -- 首次事件时间
    updated_at       TEXT    NOT NULL DEFAULT (datetime('now','localtime')), -- 最后事件时间
    deleted_at       TEXT             DEFAULT NULL       -- 软删除标记 (NULL=未删除)
);

CREATE INDEX IF NOT EXISTS idx_sessions_project ON t_sessions(project_name, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_updated ON t_sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_deleted ON t_sessions(deleted_at);

CREATE TABLE IF NOT EXISTS t_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,  -- 自增主键
    session_key     TEXT    NOT NULL,                   -- 所属会话标识 (FK → t_sessions.session_key)
    agent_label     TEXT    NOT NULL DEFAULT '',        -- 代理标签 (CC / CodeBuddy / …)
    event_type      TEXT    NOT NULL,                   -- 事件类型: pre_tool_use/post_tool_use/stop/error/notification/session_start/user_prompt_submit/subagent_stop/pre_compact
    tool_name       TEXT    NOT NULL DEFAULT '',        -- 工具名称: Bash/Edit/Read/Write/...
    tool_use_id     TEXT    NOT NULL DEFAULT '',        -- 工具调用唯一ID
    content         TEXT    NOT NULL DEFAULT '',        -- 截断显示内容 (≤60 rune)
    content_raw     TEXT    NOT NULL DEFAULT '',        -- 完整内容摘要
    attention_level TEXT    NOT NULL DEFAULT '',        -- 事件级别
    permission_mode TEXT    NOT NULL DEFAULT '',        -- 权限模式
    raw_payload     BLOB             DEFAULT NULL,     -- 完整 stdin JSON (原始字节，全量存储)
    timestamp       TEXT    NOT NULL DEFAULT (datetime('now','localtime')), -- 事件发生时间

    FOREIGN KEY (session_key) REFERENCES t_sessions(session_key)
);

CREATE INDEX IF NOT EXISTS idx_events_session_ts ON t_events(session_key, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_type       ON t_events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_timestamp  ON t_events(timestamp DESC);
