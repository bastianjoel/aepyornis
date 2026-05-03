package controller

import (
	"errors"
	"net/http"

	"github.com/AepyornisNet/aepyornis/pkg/model"
	"github.com/AepyornisNet/aepyornis/pkg/model/dto"
	"github.com/AepyornisNet/aepyornis/pkg/repository"
	"github.com/labstack/echo/v4"
	"github.com/samber/do/v2"
)

type NotificationController interface {
	GetNotifications(c echo.Context) error
}

type notificationController struct {
	notificationRepo repository.Notification
}

func NewNotificationController(injector do.Injector) NotificationController {
	return &notificationController{
		notificationRepo: do.MustInvoke[repository.Notification](injector),
	}
}

// GetNotifications returns all unread notifications of the user
// @Summary      Get user notifications
// @Tags         notification
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Produce      json
// @Success      200  {object}  dto.Response[[]model.Notification]
// @Failure      400  {object}  dto.Response[any]
// @Failure      500  {object}  dto.Response[any]
// @Router       /notifications [get]
func (nc *notificationController) GetNotifications(c echo.Context) error {
	unread, err := nc.notificationRepo.GetUnread(c.Request().Context())
	if err != nil {
		return renderApiError(c, http.StatusInternalServerError, errors.New("could not read notifications"))
	}

	resp := dto.Response[[]model.Notification]{
		Results: unread,
	}

	return c.JSON(http.StatusOK, resp)
}
