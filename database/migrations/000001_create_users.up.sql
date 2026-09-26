CREATE TABLE users (
    id            BIGSERIAL    NOT NULL CONSTRAINT pk_users PRIMARY KEY,
    name          VARCHAR(150) NOT NULL,
    email         VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_users_email ON users (email);
