-- Quick sanity checks against a seeded development database.

SELECT name, collation_name FROM sys.databases WHERE name = DB_NAME();

SELECT t.name AS table_name, p.rows AS row_count
FROM sys.tables t
JOIN sys.partitions p ON p.object_id = t.object_id AND p.index_id IN (0, 1)
ORDER BY t.name;

SELECT i.name AS index_name, t.name AS table_name, i.is_unique
FROM sys.indexes i
JOIN sys.tables t ON t.object_id = i.object_id
WHERE i.index_id > 0
ORDER BY t.name, i.name;

-- Totals per type for the current month, mirroring what the dashboard reports.
SELECT transaction_type, COUNT(*) AS transaction_count, SUM(amount) AS total_amount
FROM transactions
WHERE transaction_date >= DATEFROMPARTS(YEAR(SYSUTCDATETIME()), MONTH(SYSUTCDATETIME()), 1)
GROUP BY transaction_type
ORDER BY transaction_type;
