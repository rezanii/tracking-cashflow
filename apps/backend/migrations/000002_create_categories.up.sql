CREATE TABLE categories (
    id          BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_categories PRIMARY KEY,
    user_id     BIGINT        NOT NULL,
    name        NVARCHAR(100) NOT NULL,
    type        VARCHAR(10)   NOT NULL,
    description NVARCHAR(255) NULL,
    is_active   BIT           NOT NULL CONSTRAINT df_categories_is_active  DEFAULT (1),
    created_at  DATETIME2(3)  NOT NULL CONSTRAINT df_categories_created_at DEFAULT (SYSUTCDATETIME()),
    updated_at  DATETIME2(3)  NOT NULL CONSTRAINT df_categories_updated_at DEFAULT (SYSUTCDATETIME()),
    CONSTRAINT fk_categories_user  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ck_categories_type  CHECK (type IN ('INCOME', 'EXPENSE'))
);

CREATE UNIQUE INDEX uq_categories_user_type_name ON categories (user_id, type, name);

CREATE INDEX ix_categories_user_type ON categories (user_id, type, is_active);
