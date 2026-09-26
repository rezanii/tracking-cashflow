DROP INDEX ix_transactions_parent     ON transactions;

DROP INDEX ix_transactions_to_account ON transactions;

DROP INDEX ix_transactions_account    ON transactions;

ALTER TABLE transactions DROP CONSTRAINT ck_transactions_parent_not_self;

ALTER TABLE transactions DROP CONSTRAINT ck_transactions_transfer_accounts;

ALTER TABLE transactions DROP CONSTRAINT fk_transactions_parent;

ALTER TABLE transactions DROP CONSTRAINT fk_transactions_to_account;

ALTER TABLE transactions DROP CONSTRAINT fk_transactions_account;

ALTER TABLE transactions DROP COLUMN parent_id, to_account_id, account_id;
