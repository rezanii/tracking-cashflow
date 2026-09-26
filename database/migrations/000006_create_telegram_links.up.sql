-- Binds one Telegram chat to one application user. Without this row the bot answers
-- nothing but "not linked": a chat id alone proves no identity, and anyone can find a bot.
CREATE TABLE telegram_links (
    id         BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_telegram_links PRIMARY KEY,
    user_id    BIGINT        NOT NULL,
    chat_id    BIGINT        NOT NULL,
    username   NVARCHAR(100) NULL,
    chat_title NVARCHAR(150) NULL,
    linked_at  DATETIME2(3)  NOT NULL CONSTRAINT df_telegram_links_linked_at DEFAULT (SYSUTCDATETIME()),
    created_at DATETIME2(3)  NOT NULL CONSTRAINT df_telegram_links_created_at DEFAULT (SYSUTCDATETIME()),
    updated_at DATETIME2(3)  NOT NULL CONSTRAINT df_telegram_links_updated_at DEFAULT (SYSUTCDATETIME()),
    CONSTRAINT fk_telegram_links_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- One chat belongs to one user, and one user has one chat: both directions are unique so a
-- report can never be delivered to a second, unnoticed chat.
CREATE UNIQUE INDEX uq_telegram_links_chat ON telegram_links (chat_id);

CREATE UNIQUE INDEX uq_telegram_links_user ON telegram_links (user_id);

-- Short-lived single-use codes. The user asks the API for one while authenticated, then
-- sends it to the bot, which proves the same person controls both sides.
CREATE TABLE telegram_pairing_codes (
    id         BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT pk_telegram_pairing_codes PRIMARY KEY,
    user_id    BIGINT       NOT NULL,
    code       VARCHAR(32)  NOT NULL,
    expires_at DATETIME2(3) NOT NULL,
    used_at    DATETIME2(3) NULL,
    created_at DATETIME2(3) NOT NULL CONSTRAINT df_telegram_pairing_codes_created_at DEFAULT (SYSUTCDATETIME()),
    CONSTRAINT fk_telegram_pairing_codes_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_telegram_pairing_codes_code ON telegram_pairing_codes (code);

CREATE INDEX ix_telegram_pairing_codes_user ON telegram_pairing_codes (user_id, expires_at DESC);
