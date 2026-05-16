package service

import (
	"context"
	"fmt"

	"github.com/AepyornisNet/aepyornis/pkg/config"
	"github.com/AepyornisNet/aepyornis/pkg/model"
	"github.com/AepyornisNet/aepyornis/pkg/notification"
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
	Send(ctx context.Context, user *model.User, nfy BaseNotification) error
}

type notificationService struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewNotificationService(injector do.Injector) (NotificationService, error) {
	return &notificationService{
		cfg: do.MustInvoke[*config.Config](injector),
		db:  do.MustInvoke[*gorm.DB](injector),
	}, nil
}

func (s *notificationService) SendRaw(ctx context.Context, user *model.User, subject string, message string) error {
	nfy := model.Notification{
		UserID:  user.ID,
		Type:    "raw",
		Subject: subject,
		Msg:     message,
	}

	err := gorm.G[model.Notification](s.db).Create(ctx, &nfy)
	if err != nil {
		return fmt.Errorf("could not save notification: %w", err)
	}

	n := notify.NewWithServices(s.getEmailService(user.Email)...)
	if err := n.Send(ctx, subject, message); err != nil {
		return err
	}

	return nil
}

func (s *notificationService) Send(ctx context.Context, user *model.User, in BaseNotification) error {
	if in.AllowDB() {
		nfy := model.Notification{
			UserID:  user.ID,
			Type:    in.GetType(),
			Subject: in.GetSubject(),
			Msg:     in.GetMessage(),
			Meta:    in.GetMeta(),
		}

		err := gorm.G[model.Notification](s.db).Create(ctx, &nfy)
		if err != nil {
			return fmt.Errorf("could not save notification: %w", err)
		}
	}

	services := []notify.Notifier{}
	if in.AllowEmail() {
		services = append(services, s.getEmailService(user.Email)...)
	}

	// TODO: Add webpush config
	// if in.AllowWebpush() {
	// }

	if len(services) > 0 {
		n := notify.NewWithServices(services...)
		if err := n.Send(ctx, in.GetSubject(), in.GetMessage()); err != nil {
			return err
		}
	}

	return nil
}

func (s *notificationService) getEmailService(receiver string) []notify.Notifier {
	services := []notify.Notifier{}

	if s.cfg.SmtpHost != "" && s.cfg.MailSenderAddress != "" {
		mailService := mail.New(s.cfg.MailSenderAddress, s.cfg.SmtpHost)
		mailService.AddReceivers(receiver)
		services = append(services, mailService)
	} else if s.cfg.MailjetPublicKey != "" && s.cfg.MailjetPrivateKey != "" {
		mailService := notification.NewMailjet(s.cfg.MailjetPublicKey, s.cfg.MailjetPrivateKey, s.cfg.MailSenderAddress, s.cfg.MailSenderName)
		mailService.AddReceivers(receiver)
		services = append(services, mailService)
	}

	return services
}
