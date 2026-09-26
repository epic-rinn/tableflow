-- Fixed-window counter. $1 bucket, $2 window. Returns attempts in the
-- current window (including this one) and when the window started.
INSERT INTO auth_throttle AS t (bucket, window_start, attempts)
VALUES ($1, now(), 1)
ON CONFLICT (bucket) DO UPDATE SET
    window_start = CASE WHEN t.window_start <= now() - $2::interval THEN now() ELSE t.window_start END,
    attempts     = CASE WHEN t.window_start <= now() - $2::interval THEN 1 ELSE t.attempts + 1 END
RETURNING attempts, extract(epoch FROM (window_start + $2::interval - now()))::float8
