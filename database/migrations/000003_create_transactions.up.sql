CREATE TABLE transactions (
    id               BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_transactions PRIMARY KEY,
    user_id          BIGINT         NOT NULL,
    transaction_date DATE           NOT NULL,
    transaction_type VARCHAR(10)    NOT NULL,
    category_id      BIGINT         NULL,
    amount           DECIMAL(18, 2) NOT NULL,
    description      NVARCHAR(500)  NULL,
    reference_number NVARCHAR(100)  NULL,
    created_at       DATETIME2(3)   NOT NULL CONSTRAINT df_transactions_created_at DEFAULT (SYSUTCDATETIME()),
    updated_at       DATETIME2(3)   NOT NULL CONSTRAINT df_transactions_updated_at DEFAULT (SYSUTCDATETIME()),
    CONSTRAINT fk_transactions_user     FOREIGN KEY (user_id)     REFERENCES users (id)      ON DELETE CASCADE,
    CONSTRAINT fk_transactions_category FOREIGN KEY (category_id) REFERENCES categories (id) ON DELETE NO ACTION,
    CONSTRAINT ck_transactions_type     CHECK (transaction_type IN ('INCOME', 'EXPENSE', 'TRANSFER')),
    CONSTRAINT ck_transactions_amount   CHECK (amount > 0),
    CONSTRAINT ck_transactions_category CHECK (
        (transaction_type = 'TRANSFER' AND category_id IS NULL)
        OR (transaction_type IN ('INCOME', 'EXPENSE') AND category_id IS NOT NULL)
    )
);

CREATE INDEX ix_transactions_user_date      ON transactions (user_id, transaction_date DESC);

CREATE INDEX ix_transactions_user_type_date ON transactions (user_id, transaction_type, transaction_date);

CREATE INDEX ix_transactions_user_category  ON transactions (user_id, category_id);

CREATE INDEX ix_transactions_reference      ON transactions (user_id, reference_number);
