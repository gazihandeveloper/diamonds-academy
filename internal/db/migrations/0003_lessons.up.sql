CREATE TABLE IF NOT EXISTS lessons (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    day_id      INTEGER NOT NULL REFERENCES days(id) ON DELETE CASCADE,
    kind        TEXT    NOT NULL CHECK (kind IN ('lesson','file','quiz')),
    position    INTEGER NOT NULL DEFAULT 0,
    title       TEXT    NOT NULL,
    video_url   TEXT    NOT NULL DEFAULT '',
    file_url    TEXT    NOT NULL DEFAULT '',
    content     TEXT    NOT NULL DEFAULT '',
    published   INTEGER NOT NULL DEFAULT 1 CHECK (published IN (0,1)),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_lessons_day_id_position
    ON lessons(day_id, position);
