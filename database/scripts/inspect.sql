-- Quick sanity checks against a seeded development database.
-- Run with: psql "$DATABASE_URL" -f database/scripts/inspect.sql

SELECT current_database() AS database_name, pg_encoding_to_char(encoding) AS encoding, datcollate AS collation
FROM pg_database
WHERE datname = current_database();

-- reltuples is an estimate maintained by ANALYZE, which is enough for a sanity check and far
-- cheaper than counting every table.
SELECT relname AS table_name, reltuples::BIGINT AS estimated_rows
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'r' AND n.nspname = 'public'
ORDER BY relname;

SELECT indexname AS index_name, tablename AS table_name, indexdef LIKE 'CREATE UNIQUE%' AS is_unique
FROM pg_indexes
WHERE schemaname = 'public'
ORDER BY tablename, indexname;

-- Totals per type for the current month, mirroring what the dashboard reports.
SELECT transaction_type, COUNT(*) AS transaction_count, SUM(amount) AS total_amount
FROM transactions
WHERE transaction_date >= date_trunc('month', NOW())::DATE
GROUP BY transaction_type
ORDER BY transaction_type;

-- The accounts the daily report is built from, with the most recent observed balance.
SELECT a.account_type, a.name, a.is_active, s.as_of_date, s.actual_balance
FROM accounts a
LEFT JOIN LATERAL (
    SELECT as_of_date, actual_balance
    FROM account_balance_snapshots
    WHERE account_id = a.id
    ORDER BY as_of_date DESC
    LIMIT 1
) s ON TRUE
ORDER BY a.account_type, a.name;
