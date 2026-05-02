CREATE TABLE IF NOT EXISTS watch_progress (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL,
    day_no          INTEGER NOT NULL,
    slot            TEXT    NOT NULL CHECK (slot IN ('l1','l2','l3')),
    position        REAL    NOT NULL DEFAULT 0,
    duration        REAL    NOT NULL DEFAULT 0,
    seconds_watched REAL    NOT NULL DEFAULT 0,
    percent         REAL    NOT NULL DEFAULT 0,
    completed       INTEGER NOT NULL DEFAULT 0,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, day_no, slot)
);

CREATE INDEX IF NOT EXISTS idx_watch_progress_user_day ON watch_progress(user_id, day_no);
CREATE INDEX IF NOT EXISTS idx_watch_progress_updated ON watch_progress(updated_at);
