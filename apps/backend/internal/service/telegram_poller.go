package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
)

// pollBackoff is how long the loop waits after a failed getUpdates before trying again, so a
// Telegram outage does not turn into a request flood.
const pollBackoff = 5 * time.Second

// TelegramPoller pulls updates instead of receiving them. It is what makes the bot work on a
// laptop or behind NAT, where Telegram cannot reach a webhook.
type TelegramPoller struct {
	client  TelegramClient
	service TelegramService
	// offset is the id after the last handled update; Telegram uses it to acknowledge.
	offset int64
}

func NewTelegramPoller(client TelegramClient, service TelegramService) *TelegramPoller {
	return &TelegramPoller{client: client, service: service}
}

// Run blocks until the context is cancelled. Each update is handled before the offset moves
// past it, so a crash re-delivers the update rather than losing it.
func (p *TelegramPoller) Run(ctx context.Context) {
	slog.Info("telegram poller started")
	defer slog.Info("telegram poller stopped")

	for {
		if ctx.Err() != nil {
			return
		}

		updates, err := p.client.GetUpdates(ctx, p.offset)
		if err != nil {
			// A cancelled context during shutdown is not a failure worth logging as one.
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return
			}
			slog.Error("telegram getUpdates failed", "error", err)
			if !sleepCtx(ctx, pollBackoff) {
				return
			}
			continue
		}

		p.handle(ctx, updates)
	}
}

func (p *TelegramPoller) handle(ctx context.Context, updates []dto.TelegramUpdate) {
	for _, update := range updates {
		if ctx.Err() != nil {
			return
		}
		p.service.HandleUpdate(ctx, update)
		if update.UpdateID >= p.offset {
			p.offset = update.UpdateID + 1
		}
	}
}

// sleepCtx waits, and reports false when the context ended first.
func sleepCtx(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
