CREATE TABLE categories (
    id          BIGSERIAL    NOT NULL CONSTRAINT pk_categories PRIMARY KEY,
    user_id     BIGINT       NOT NULL,
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(10)  NOT NULL,
    description VARCHAR(255) NULL,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_categories_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ck_categories_type CHECK (type IN ('INCOME', 'EXPENSE'))
);

CREATE UNIQUE INDEX uq_categories_user_type_name ON categories (user_id, type, name);

CREATE INDEX ix_categories_user_type ON categories (user_id, type, is_active);
