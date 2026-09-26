package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

type TelegramRepository interface {
	WithTx(tx *gorm.DB) TelegramRepository
	CreatePairingCode(ctx context.Context, code *model.TelegramPairingCode) error
	FindPairingCode(ctx context.Context, code string) (*model.TelegramPairingCode, error)
	// ConsumePairingCode marks the code used and reports whether it won the race. A second
	// caller with the same code gets false rather than a second link.
	ConsumePairingCode(ctx context.Context, id int64, usedAt time.Time) (bool, error)
	UpsertLink(ctx context.Context, link *model.TelegramLink) error
	FindLinkByUser(ctx context.Context, userID int64) (*model.TelegramLink, error)
	FindLinkByChat(ctx context.Context, chatID int64) (*model.TelegramLink, error)
	DeleteLinkByUser(ctx context.Context, userID int64) error
	DeleteLinkByChat(ctx context.Context, chatID int64) error
}

type telegramRepository struct {
	db *gorm.DB
}

func NewTelegramRepository(db *gorm.DB) TelegramRepository {
	return &telegramRepository{db: db}
}

func (r *telegramRepository) WithTx(tx *gorm.DB) TelegramRepository {
	return &telegramRepository{db: tx}
}

func (r *telegramRepository) CreatePairingCode(ctx context.Context, code *model.TelegramPairingCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}

func (r *telegramRepository) FindPairingCode(ctx context.Context, code string) (*model.TelegramPairingCode, error) {
	var pairing model.TelegramPairingCode
	err := r.db.WithContext(ctx).Where("code = ?", code).Take(&pairing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pairing, nil
}

func (r *telegramRepository) ConsumePairingCode(ctx context.Context, id int64, usedAt time.Time) (bool, error) {
	// used_at IS NULL in the WHERE clause is what makes this single-use: the update itself
	// is the claim, so two concurrent /start commands cannot both succeed.
	result := r.db.WithContext(ctx).
		Model(&model.TelegramPairingCode{}).
		Where("id = ? AND used_at IS NULL AND expires_at > ?", id, usedAt).
		Update("used_at", usedAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// UpsertLink relinks an existing user or chat rather than failing on the unique indexes, so
// re-pairing from a new chat replaces the old one instead of leaving two live destinations.
func (r *telegramRepository) UpsertLink(ctx context.Context, link *model.TelegramLink) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? OR chat_id = ?", link.UserID, link.ChatID).
			Delete(&model.TelegramLink{}).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.Returning{}).Create(link).Error
	})
}

func (r *telegramRepository) FindLinkByUser(ctx context.Context, userID int64) (*model.TelegramLink, error) {
	return r.findLink(ctx, "user_id = ?", userID)
}

func (r *telegramRepository) FindLinkByChat(ctx context.Context, chatID int64) (*model.TelegramLink, error) {
	return r.findLink(ctx, "chat_id = ?", chatID)
}

func (r *telegramRepository) findLink(ctx context.Context, condition string, arg any) (*model.TelegramLink, error) {
	var link model.TelegramLink
	err := r.db.WithContext(ctx).Where(condition, arg).Take(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *telegramRepository) DeleteLinkByUser(ctx context.Context, userID int64) error {
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.TelegramLink{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Telegram link")
	}
	return nil
}

func (r *telegramRepository) DeleteLinkByChat(ctx context.Context, chatID int64) error {
	result := r.db.WithContext(ctx).Where("chat_id = ?", chatID).Delete(&model.TelegramLink{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Telegram link")
	}
	return nil
}
