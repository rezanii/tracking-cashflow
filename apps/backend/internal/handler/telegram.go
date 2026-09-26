package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/middleware"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/service"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

// webhookMaxBody caps what an unauthenticated endpoint will read into memory.
const webhookMaxBody = 1 << 20

// webhookWorkTimeout bounds the work one update may do before the response is sent. It is
// generous because building a report and delivering it crosses the network twice, and because
// Telegram retrying is cheaper than losing the update.
const webhookWorkTimeout = 30 * time.Second

type TelegramHandler struct {
	telegram service.TelegramService
	// webhookSecret is compared against the header Telegram sends. Empty means the webhook
	// endpoint is not accepting anything.
	webhookSecret string
}

func NewTelegramHandler(telegram service.TelegramService, webhookSecret string) *TelegramHandler {
	return &TelegramHandler{telegram: telegram, webhookSecret: webhookSecret}
}

// PairingCode issues a single-use code for connecting a Telegram chat.
//
//	@Summary		Issue Telegram pairing code
//	@Description	Returns a short-lived single-use code. Send "/start <code>" to the bot from the
//	@Description	chat that should receive reports. Until a chat is paired the bot answers nothing.
//	@Tags			Telegram
//	@Produce		json
//	@Security		BearerAuth
//	@Success		201	{object}	utils.Envelope{data=dto.TelegramPairingCodeResponse}
//	@Failure		401	{object}	utils.Envelope
//	@Router			/telegram/pairing-code [post]
func (h *TelegramHandler) PairingCode(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	code, err := h.telegram.IssuePairingCode(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.Created(w, "Pairing code created successfully", code)
}

// Link reports whether a chat is connected.
//
//	@Summary		Telegram link status
//	@Tags			Telegram
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	utils.Envelope{data=dto.TelegramLinkResponse}
//	@Failure		401	{object}	utils.Envelope
//	@Router			/telegram/link [get]
func (h *TelegramHandler) Link(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	status, err := h.telegram.LinkStatus(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", status)
}

// Unlink disconnects the chat.
//
//	@Summary		Unlink Telegram chat
//	@Tags			Telegram
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	utils.Envelope
//	@Failure		404	{object}	utils.Envelope
//	@Router			/telegram/link [delete]
func (h *TelegramHandler) Unlink(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	if err := h.telegram.Unlink(r.Context(), userID); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Telegram chat unlinked successfully", nil)
}

// SendDailyReport pushes the daily cash flow report to the linked chat.
//
//	@Summary		Send daily report to Telegram
//	@Tags			Telegram
//	@Produce		json
//	@Security		BearerAuth
//	@Param			date	query		string	false	"Report date, YYYY-MM-DD. Defaults to today."
//	@Success		200		{object}	utils.Envelope{data=dto.TelegramSendResponse}
//	@Failure		422		{object}	utils.Envelope	"No chat is linked"
//	@Router			/telegram/send/daily-report [post]
func (h *TelegramHandler) SendDailyReport(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	date, err := reportDate(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	result, err := h.telegram.SendDailyReport(r.Context(), userID, date)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Report sent successfully", result)
}

// Webhook receives updates from Telegram.
//
//	@Summary		Telegram webhook
//	@Description	Called by Telegram, not by clients. The X-Telegram-Bot-Api-Secret-Token header
//	@Description	must match TELEGRAM_WEBHOOK_SECRET, so knowing the URL is not enough to post
//	@Description	forged updates.
//	@Tags			Telegram
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	utils.Envelope
//	@Failure		401	{object}	utils.Envelope
//	@Router			/telegram/webhook [post]
func (h *TelegramHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	// No secret configured means the endpoint is closed rather than open to everyone.
	if h.webhookSecret == "" {
		utils.Error(w, http.StatusNotFound, "Endpoint not found", nil)
		return
	}

	provided := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	// Constant-time comparison keeps the check from leaking the secret one byte at a time.
	if subtle.ConstantTimeCompare([]byte(provided), []byte(h.webhookSecret)) != 1 {
		utils.Error(w, http.StatusUnauthorized, "Invalid webhook secret", nil)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, webhookMaxBody))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "Request body could not be read", nil)
		return
	}

	var update dto.TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		// Telegram retries anything that is not a 2xx, and a malformed update will never
		// parse, so it is acknowledged and dropped.
		slog.Warn("telegram webhook received an unparsable update", "error", err)
		utils.OK(w, "Ignored", nil)
		return
	}

	// Handled before responding, deliberately. Answering first and doing the work in a
	// detached goroutine is faster on a long-lived server, but on a serverless host the
	// instance is frozen the moment the response is written and the goroutine never finishes —
	// the update is acknowledged and silently dropped. Telegram tolerates a slow response and
	// retries on a timeout, which is the far better failure mode.
	ctx, cancel := context.WithTimeout(r.Context(), webhookWorkTimeout)
	defer cancel()
	h.telegram.HandleUpdate(ctx, update)

	utils.OK(w, "Accepted", nil)
}

// reportDate reads the optional date query parameter, defaulting to today.
func reportDate(r *http.Request) (time.Time, error) {
	raw := r.URL.Query().Get("date")
	if raw == "" {
		return time.Now().UTC(), nil
	}
	date, err := utils.ParseDate(raw)
	if err != nil {
		return time.Time{}, utils.NewFieldError("Validation failed", map[string]string{"date": err.Error()})
	}
	return date, nil
}
