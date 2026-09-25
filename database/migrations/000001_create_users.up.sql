CREATE TABLE users (
    id            BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_users PRIMARY KEY,
    name          NVARCHAR(150)  NOT NULL,
    email         NVARCHAR(255)  NOT NULL,
    password_hash NVARCHAR(255)  NOT NULL,
    is_active     BIT            NOT NULL CONSTRAINT df_users_is_active  DEFAULT (1),
    created_at    DATETIME2(3)   NOT NULL CONSTRAINT df_users_created_at DEFAULT (SYSUTCDATETIME()),
    updated_at    DATETIME2(3)   NOT NULL CONSTRAINT df_users_updated_at DEFAULT (SYSUTCDATETIME())
);

CREATE UNIQUE INDEX uq_users_email ON users (email);
