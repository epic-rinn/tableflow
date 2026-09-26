DELETE FROM auth_throttle WHERE window_start < now() - interval '1 day'
