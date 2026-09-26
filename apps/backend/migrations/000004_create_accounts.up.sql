CREATE TABLE accounts (
    id              BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_accounts PRIMARY KEY,
    user_id         BIGINT         NOT NULL,
    name            NVARCHAR(100)  NOT NULL,
    account_type    VARCHAR(20)    NOT NULL,
    opening_balance DECIMAL(18, 2) NOT NULL CONSTRAINT df_accounts_opening_balance DEFAULT (0),
    description     NVARCHAR(255)  NULL,
    is_active       BIT            NOT NULL CONSTRAINT df_accounts_is_active DEFAULT (1),
    created_at      DATETIME2(3)   NOT NULL CONSTRAINT df_accounts_created_at DEFAULT (SYSUTCDATETIME()),
    updated_at      DATETIME2(3)   NOT NULL CONSTRAINT df_accounts_updated_at DEFAULT (SYSUTCDATETIME()),
    CONSTRAINT fk_accounts_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    -- The type is what drives the report layout, so an unknown value would silently
    -- drop an account out of every section. The database rejects it instead.
    CONSTRAINT ck_accounts_type CHECK (account_type IN ('CASH_FLOW', 'WALLET', 'BANK', 'CREDIT_CARD', 'SAVINGS'))
);

CREATE UNIQUE INDEX uq_accounts_user_name ON accounts (user_id, name);

CREATE INDEX ix_accounts_user_type ON accounts (user_id, account_type, is_active);

-- An observed balance, read off a banking app or counted by hand. It cannot be derived
-- from the transactions, and the gap between the two is exactly what the report reports
-- as "belum tercatat".
CREATE TABLE account_balance_snapshots (
    id             BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_account_balance_snapshots PRIMARY KEY,
    user_id        BIGINT         NOT NULL,
    account_id     BIGINT         NOT NULL,
    as_of_date     DATE           NOT NULL,
    actual_balance DECIMAL(18, 2) NOT NULL,
    note           NVARCHAR(255)  NULL,
    created_at     DATETIME2(3)   NOT NULL CONSTRAINT df_account_balance_snapshots_created_at DEFAULT (SYSUTCDATETIME()),
    updated_at     DATETIME2(3)   NOT NULL CONSTRAINT df_account_balance_snapshots_updated_at DEFAULT (SYSUTCDATETIME()),
    -- The account cascade already removes these rows with their account. The user FK is
    -- NO ACTION because two cascade paths to users is not allowed in SQL Server.
    CONSTRAINT fk_account_balance_snapshots_user    FOREIGN KEY (user_id)    REFERENCES users (id)    ON DELETE NO ACTION,
    CONSTRAINT fk_account_balance_snapshots_account FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_account_balance_snapshots_account_date
    ON account_balance_snapshots (account_id, as_of_date);

CREATE INDEX ix_account_balance_snapshots_user_date
    ON account_balance_snapshots (user_id, as_of_date DESC);
