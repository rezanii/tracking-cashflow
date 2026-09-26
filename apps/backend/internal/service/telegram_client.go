package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
)

// Long polling holds the request open, so the client timeout has to outlast it.
const (
	telegramRequestTimeout = 100 * time.Second
	telegramPollTimeout    = 50
)

// TelegramClient is the slice of the Bot API this service needs. It is an interface so the
// tests can drive the command handling without reaching the network.
type TelegramClient interface {
	SendMessage(ctx context.Context, chatID int64, text string, markdown bool) error
	GetUpdates(ctx context.Context, offset int64) ([]dto.TelegramUpdate, error)
	SetWebhook(ctx context.Context, webhookURL, secret string) error
	DeleteWebhook(ctx context.Context) error
	GetMe(ctx context.Context) (string, error)
}

type telegramClient struct {
	token   string
	baseURL string
	http    *http.Client
}

func NewTelegramClient(token, baseURL string) TelegramClient {
	return &telegramClient{
		token:   token,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: telegramRequestTimeout},
	}
}

// telegramResponse is the envelope every Bot API method returns.
type telegramResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Result      json.RawMessage `json:"result"`
}

func (c *telegramClient) method(name string) string {
	// The token is a credential, so it stays out of logs: only the method name is ever
	// reported in an error.
	return fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.token, name)
}

func (c *telegramClient) call(ctx context.Context, method string, payload any, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", method, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.method(method), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s request: %w", method, err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		// url.Error repeats the full URL, which carries the bot token. Only the method is
		// kept so a token cannot leak into a log line.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return fmt.Errorf("call %s: %w", method, urlErr.Err)
		}
		return fmt.Errorf("call %s: %w", method, err)
	}
	defer response.Body.Close()

	var envelope telegramResponse
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	if !envelope.OK {
		return fmt.Errorf("telegram %s failed with %d: %s", method, envelope.ErrorCode, envelope.Description)
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode %s result: %w", method, err)
	}
	return nil
}

func (c *telegramClient) SendMessage(ctx context.Context, chatID int64, text string, markdown bool) error {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if markdown {
		payload["parse_mode"] = "MarkdownV2"
	}
	return c.call(ctx, "sendMessage", payload, nil)
}

func (c *telegramClient) GetUpdates(ctx context.Context, offset int64) ([]dto.TelegramUpdate, error) {
	var updates []dto.TelegramUpdate
	payload := map[string]any{
		"offset":  offset,
		"timeout": telegramPollTimeout,
		// Only the two kinds this service acts on are requested; everything else is left
		// unfetched rather than received and discarded.
		"allowed_updates": []string{"message", "edited_message"},
	}
	if err := c.call(ctx, "getUpdates", payload, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *telegramClient) SetWebhook(ctx context.Context, webhookURL, secret string) error {
	return c.call(ctx, "setWebhook", map[string]any{
		"url":             webhookURL,
		"secret_token":    secret,
		"allowed_updates": []string{"message", "edited_message"},
		// Anything queued while the webhook was off is stale by definition.
		"drop_pending_updates": true,
	}, nil)
}

func (c *telegramClient) DeleteWebhook(ctx context.Context) error {
	return c.call(ctx, "deleteWebhook", map[string]any{"drop_pending_updates": true}, nil)
}

// GetMe returns the bot username, which is enough to prove the token is usable.
func (c *telegramClient) GetMe(ctx context.Context) (string, error) {
	var result struct {
		Username string `json:"username"`
	}
	if err := c.call(ctx, "getMe", map[string]any{}, &result); err != nil {
		return "", err
	}
	return result.Username, nil
}
