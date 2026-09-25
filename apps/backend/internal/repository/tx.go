package repository

import (
	"context"

	"gorm.io/gorm"
)

// TxManager runs a unit of work inside a single SQL transaction. Repositories join it by
// being rebound with WithTx, so a service can compose several writes atomically.
type TxManager interface {
	Run(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type txManager struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) TxManager {
	return &txManager{db: db}
}

func (m *txManager) Run(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(fn)
}
