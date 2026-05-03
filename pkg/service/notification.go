package service

import (
	"context"
	"fmt"

	"github.com/AepyornisNet/aepyornis/pkg/config"
	"github.com/AepyornisNet/aepyornis/pkg/model"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/mail"
	"github.com/samber/do/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type BaseNotification interface {
	GetType() string
	GetSubject() string
	GetMessage() string
	GetMeta() *datatypes.JSON

	AllowDB() bool
	AllowEmail() bool
	AllowWebpush() bool
}

type NotificationService interface {
	SendRaw(ctx context.Context, user *model.User, subject string, message string) error
	Send(ctx context.Context, user *model.User, notification BaseNotification) error
}

type notificationService struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewNotificationService(injector do.Injector) (NotificationService, error) {
	return &notificationService{
		cfg: do.MustInvoke[*config.Config](injector),
		db:  do.MustInvoke[*gorm.DB](injector)}, nil
}

func (s *notificationService) SendRaw(ctx context.Context, user *model.User, subject string, message string) error {
	notification := model.Notification{
		UserID:  user.ID,
		Type:    "raw",
		Subject: subject,
		Msg:     message,
	}

	err := gorm.G[model.Notification](s.db).Create(ctx, &notification)
	if err != nil {
		return fmt.Errorf("could not save notification: %w", err)
	}

	if s.cfg.SmtpHost != "" && s.cfg.SmtpSender != "" {
		mailService := mail.New(s.cfg.SmtpSender, s.cfg.SmtpHost)
		mailService.AddReceivers(user.Email)

		n := notify.NewWithServices(mailService)
		if err := n.Send(ctx, subject, message); err != nil {
			return err
		}
	}

	return nil
}

func (s *notificationService) Send(ctx context.Context, user *model.User, in BaseNotification) error {
	if in.AllowDB() {
		notification := model.Notification{
			UserID:  user.ID,
			Type:    in.GetType(),
			Subject: in.GetSubject(),
			Msg:     in.GetMessage(),
			Meta:    in.GetMeta(),
		}

		err := gorm.G[model.Notification](s.db).Create(ctx, &notification)
		if err != nil {
			return fmt.Errorf("could not save notification: %w", err)
		}
	}

	services := []notify.Notifier{}
	if in.AllowEmail() {
		if s.cfg.SmtpHost != "" && s.cfg.SmtpSender != "" {
			mailService := mail.New(s.cfg.SmtpSender, s.cfg.SmtpHost)
			mailService.AddReceivers(user.Email)
			services = append(services, mailService)
		}
	}

	if in.AllowWebpush() {
		// TODO: Add webpush config
	}

	if len(services) > 0 {
		n := notify.NewWithServices(services...)
		if err := n.Send(ctx, in.GetSubject(), in.GetMessage()); err != nil {
			return err
		}
	}

	return nil
}
