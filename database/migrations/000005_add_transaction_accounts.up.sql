-- account_id is where the money moved, to_account_id is the other side of a transfer, and
-- parent_id lets one recorded amount be broken into the items it was actually spent on
-- (a cash withdrawal split into the things bought with it).
ALTER TABLE transactions ADD
    account_id    BIGINT NULL CONSTRAINT fk_transactions_account    FOREIGN KEY REFERENCES accounts (id),
    to_account_id BIGINT NULL CONSTRAINT fk_transactions_to_account FOREIGN KEY REFERENCES accounts (id),
    parent_id     BIGINT NULL CONSTRAINT fk_transactions_parent     FOREIGN KEY REFERENCES transactions (id);

-- The statements below are wrapped in EXEC because SQL Server compiles a batch as a whole:
-- the columns added above are not visible to the rest of this file otherwise.

-- A destination account only makes sense on a transfer, and a transfer to itself would
-- create money out of nothing on both sides of the report.
EXEC(N'
ALTER TABLE transactions ADD CONSTRAINT ck_transactions_transfer_accounts CHECK (
    to_account_id IS NULL
    OR (transaction_type = ''TRANSFER'' AND account_id IS NOT NULL AND to_account_id <> account_id)
)');

EXEC(N'
ALTER TABLE transactions ADD CONSTRAINT ck_transactions_parent_not_self CHECK (
    parent_id IS NULL OR parent_id <> id
)');

EXEC(N'CREATE INDEX ix_transactions_account    ON transactions (user_id, account_id, transaction_date)');

EXEC(N'CREATE INDEX ix_transactions_to_account ON transactions (user_id, to_account_id, transaction_date)');

EXEC(N'CREATE INDEX ix_transactions_parent     ON transactions (parent_id)');
