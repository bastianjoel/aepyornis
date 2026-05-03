package repository

import (
	"context"

	"github.com/AepyornisNet/aepyornis/pkg/model"
	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

type Notification interface {
	GetUnread(ctx context.Context) ([]model.Notification, error)
}

type notificationRepository struct {
	db *gorm.DB
}

func NewNotification(injector do.Injector) (Notification, error) {
	return &notificationRepository{db: do.MustInvoke[*gorm.DB](injector)}, nil
}

func (r *notificationRepository) GetUnread(ctx context.Context) ([]model.Notification, error) {
	notifications, err := gorm.G[model.Notification](r.db).Where("read_at IS NULL").Find(ctx)
	if err != nil {
		return nil, err
	}

	return notifications, nil
}
