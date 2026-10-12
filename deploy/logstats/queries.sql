-- queries.sql — DuckDB cheat-sheet for the solo-relay access log.
--
-- Usage:
--   duckdb            # then paste the view + any query below
--   duckdb -c "<query>"
--
-- The log is NDJSON written by relay-go's internal/accesslog. The glob covers
-- rotated files too (access.log.1, access.log.2.gz — DuckDB reads .gz
-- transparently), and union_by_name tolerates the different event schemas.
--
-- Event schemas (all have ts + event):
--   http_req: method, path, status, cost_ms, ip, ua
--   ws_open:  server_id, role, conn_id, version, ip, ua, origin
--   ws_close: server_id, role, conn_id, dur_ms, msgs
--
-- ts is RFC3339; casting to TIMESTAMP keeps the server's local wall-clock,
-- which is what you want for "today / per day / per hour" questions.

CREATE OR REPLACE VIEW relay_log AS
SELECT
    CAST(ts AS TIMESTAMP) AS ts,
    event,
    method, path, status, cost_ms, ip, ua,
    server_id, role, conn_id, version, origin,
    dur_ms, msgs
FROM read_json('/var/log/solo-relay/access.log*',
               format = 'newline_delimited',
               union_by_name = true);

-- 1) Today: request volume and connection counts.
SELECT
    count(*) FILTER (event = 'http_req')            AS http_requests,
    count(*) FILTER (event = 'ws_open')             AS ws_connections,
    count(DISTINCT server_id) FILTER (event = 'ws_open') AS unique_servers
FROM relay_log
WHERE CAST(ts AS DATE) = current_date;

-- 2) Per day: WS sessions and their duration (from ws_close).
SELECT
    CAST(ts AS DATE)                 AS day,
    count(*)                         AS sessions,
    round(avg(dur_ms) / 1000.0, 1)   AS avg_duration_s,
    round(max(dur_ms) / 1000.0, 1)   AS max_duration_s
FROM relay_log
WHERE event = 'ws_close'
GROUP BY day
ORDER BY day DESC;

-- 3) Per day and role: message volume relayed (msgs summed over ws_close).
SELECT
    CAST(ts AS DATE)  AS day,
    role,
    count(*)          AS closed_connections,
    sum(msgs)         AS messages,
    round(avg(msgs))  AS avg_msgs_per_conn
FROM relay_log
WHERE event = 'ws_close'
GROUP BY day, role
ORDER BY day DESC, role;

-- 4) User-Agent distribution, last 7 days (http_req covers WS upgrades too,
--    since upgrade requests pass the same middleware).
SELECT
    ua,
    count(*)                                   AS requests,
    count(*) FILTER (path = '/ws')             AS of_which_ws
FROM relay_log
WHERE event = 'http_req'
  AND ts >= current_timestamp - INTERVAL 7 DAY
GROUP BY ua
ORDER BY requests DESC;

-- 5) HTTP status distribution (all time). 101 = successful WS upgrade.
SELECT
    status,
    count(*)            AS requests,
    count(DISTINCT ip)  AS unique_ips
FROM relay_log
WHERE event = 'http_req'
GROUP BY status
ORDER BY requests DESC;

-- 6) Activity by hour of day (server local time).
SELECT
    hour(ts)                                AS hour_of_day,
    count(*) FILTER (event = 'http_req')    AS http_requests,
    count(*) FILTER (event = 'ws_open')     AS ws_opens
FROM relay_log
GROUP BY hour_of_day
ORDER BY hour_of_day;
