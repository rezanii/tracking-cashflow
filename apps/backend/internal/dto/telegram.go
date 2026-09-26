package dto

import "time"

type TelegramPairingCodeResponse struct {
	Code      string    `json:"code" example:"7F3K9Q"`
	ExpiresAt time.Time `json:"expires_at"`
	// Instruction is ready to show in the UI so the user does not have to be told the
	// command format separately.
	Instruction string `json:"instruction" example:"Send /start 7F3K9Q to the bot"`
}

type TelegramLinkResponse struct {
	Linked    bool       `json:"linked" example:"true"`
	ChatID    int64      `json:"chat_id,omitempty" example:"123456789"`
	Username  string     `json:"username,omitempty" example:"rezani"`
	ChatTitle string     `json:"chat_title,omitempty" example:"Reza"`
	LinkedAt  *time.Time `json:"linked_at,omitempty"`
}

// TelegramUpdate is the slice of Telegram's update object this service acts on. Fields it
// does not use are left out on purpose: an unknown field is ignored rather than trusted.
type TelegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *TelegramMessage `json:"message"`
	// EditedMessage is handled the same way, so correcting a typo in a command still works.
	EditedMessage *TelegramMessage `json:"edited_message"`
}

type TelegramMessage struct {
	MessageID int64         `json:"message_id"`
	Text      string        `json:"text"`
	Date      int64         `json:"date"`
	Chat      *TelegramChat `json:"chat"`
	From      *TelegramUser `json:"from"`
}

type TelegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}
