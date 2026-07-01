-- Every authenticated request resolves its session via
-- device_sessions.token (GetDeviceSessionByToken: WHERE token = $1). Without
-- an index this is a sequential scan that worsens as the table grows. Add a
-- btree index so session resolution stays O(log n).
CREATE INDEX IF NOT EXISTS device_sessions_token_idx ON device_sessions (token);
