DROP INDEX IF EXISTS idx_slot_completion_user_day;
DROP INDEX IF EXISTS idx_slot_completion_user;
DROP TABLE IF EXISTS slot_completion;
-- ALTER TABLE DROP COLUMN not supported in older sqlite; leave quiz_json column.
SELECT 1;
