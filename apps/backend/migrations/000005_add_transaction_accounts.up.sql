-- account_id is where the money moved, to_account_id is the other side of a transfer, and
-- parent_id lets one recorded amount be broken into the items it was actually spent on
-- (a cash withdrawal split into the things bought with it).
ALTER TABLE transactions
    ADD COLUMN account_id    BIGINT NULL,
    ADD COLUMN to_account_id BIGINT NULL,
    ADD COLUMN parent_id     BIGINT NULL;

ALTER TABLE transactions
    ADD CONSTRAINT fk_transactions_account    FOREIGN KEY (account_id)    REFERENCES accounts (id),
    ADD CONSTRAINT fk_transactions_to_account FOREIGN KEY (to_account_id) REFERENCES accounts (id),
    ADD CONSTRAINT fk_transactions_parent     FOREIGN KEY (parent_id)     REFERENCES transactions (id);

-- A destination account only makes sense on a transfer, and a transfer to itself would
-- create money out of nothing on both sides of the report.
ALTER TABLE transactions
    ADD CONSTRAINT ck_transactions_transfer_accounts CHECK (
        to_account_id IS NULL
        OR (transaction_type = 'TRANSFER' AND account_id IS NOT NULL AND to_account_id <> account_id)
    );

ALTER TABLE transactions
    ADD CONSTRAINT ck_transactions_parent_not_self CHECK (
        parent_id IS NULL OR parent_id <> id
    );

CREATE INDEX ix_transactions_account    ON transactions (user_id, account_id, transaction_date);

CREATE INDEX ix_transactions_to_account ON transactions (user_id, to_account_id, transaction_date);

CREATE INDEX ix_transactions_parent     ON transactions (parent_id);
