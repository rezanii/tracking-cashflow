DROP INDEX ix_transactions_parent;

DROP INDEX ix_transactions_to_account;

DROP INDEX ix_transactions_account;

ALTER TABLE transactions
    DROP CONSTRAINT ck_transactions_parent_not_self,
    DROP CONSTRAINT ck_transactions_transfer_accounts,
    DROP CONSTRAINT fk_transactions_parent,
    DROP CONSTRAINT fk_transactions_to_account,
    DROP CONSTRAINT fk_transactions_account;

ALTER TABLE transactions
    DROP COLUMN parent_id,
    DROP COLUMN to_account_id,
    DROP COLUMN account_id;
