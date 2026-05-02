ALTER TABLE days ADD COLUMN quiz_json TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS slot_completion (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id      INTEGER NOT NULL,
    day_no       INTEGER NOT NULL,
    slot         TEXT    NOT NULL CHECK (slot IN ('l1','l2','l3','file','quiz')),
    completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, day_no, slot)
);
CREATE INDEX IF NOT EXISTS idx_slot_completion_user ON slot_completion(user_id);
CREATE INDEX IF NOT EXISTS idx_slot_completion_user_day ON slot_completion(user_id, day_no);
