package model

import "time"

// TelegramLink binds a Telegram chat to an application user. The bot refuses to answer a
// chat that has no link, so knowing the bot's name grants nothing.
type TelegramLink struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64     `gorm:"column:user_id;not null;uniqueIndex:uq_telegram_links_user"`
	ChatID    int64     `gorm:"column:chat_id;not null;uniqueIndex:uq_telegram_links_chat"`
	Username  string    `gorm:"column:username;size:100"`
	ChatTitle string    `gorm:"column:chat_title;size:150"`
	LinkedAt  time.Time `gorm:"column:linked_at;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (TelegramLink) TableName() string { return "telegram_links" }

// TelegramPairingCode is issued to an authenticated caller and spent once by the bot.
type TelegramPairingCode struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64      `gorm:"column:user_id;not null"`
	Code      string     `gorm:"column:code;size:32;not null;uniqueIndex:uq_telegram_pairing_codes_code"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null"`
	UsedAt    *time.Time `gorm:"column:used_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
}

func (TelegramPairingCode) TableName() string { return "telegram_pairing_codes" }

// Usable reports whether the code may still be spent at the given moment.
func (c TelegramPairingCode) Usable(now time.Time) bool {
	return c.UsedAt == nil && now.Before(c.ExpiresAt)
}
