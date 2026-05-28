package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/AepyornisNet/aepyornis/pkg/config"
	"github.com/AepyornisNet/aepyornis/pkg/model"
	"github.com/AepyornisNet/aepyornis/pkg/model/dto"
	"github.com/AepyornisNet/aepyornis/pkg/repository"
	"github.com/labstack/echo/v4"
	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

type NotificationController interface {
	GetNotifications(c echo.Context) error
	UpdateConfig(c echo.Context) error
}

type notificationController struct {
	notificationRepo repository.Notification

	cfg *config.Config
	db  *gorm.DB
}

func NewNotificationController(injector do.Injector) NotificationController {
	return &notificationController{
		notificationRepo: do.MustInvoke[repository.Notification](injector),
		cfg:              do.MustInvoke[*config.Config](injector),
		db:               do.MustInvoke[*gorm.DB](injector),
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
	user := currentUser(c)

	unread, err := nc.notificationRepo.GetUnread(c.Request().Context(), user)
	if err != nil {
		return renderApiError(c, http.StatusInternalServerError, errors.New("could not read notifications"))
	}

	resp := dto.Response[[]model.Notification]{
		Results: unread,
	}

	return c.JSON(http.StatusOK, resp)
}

// UpdateConfig updates current user's notification config
// @Summary      Update notification config
// @Tags         notification
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Accept       json
// @Produce      json
// @Success      200  {object}  dto.Response[model.UserNotificationSettings]
// @Failure      400  {object}  dto.Response[any]
// @Failure      500  {object}  dto.Response[any]
// @Router       /notifications/{type} [post]
func (nc *notificationController) UpdateConfig(c echo.Context) error {
	user := currentUser(c)

	nType := c.Param("type")
	if !slices.Contains(nc.cfg.AvailableNotificationProviders(), nType) {
		return renderApiError(c, http.StatusBadRequest, errors.New("invalid notification type"))
	}

	var updateData dto.UserNotificationSettingsData
	if err := c.Bind(&updateData); err != nil {
		return renderApiError(c, http.StatusBadRequest, err)
	}

	currentSettings, err := nc.notificationRepo.GetUserSettings(c.Request().Context(), nType, user)
	if err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	if currentSettings == nil {
		currentSettings = &model.UserNotificationSettings{
			UserID: user.ID,
			Method: nType,
		}

		gorm.G[model.UserNotificationSettings](nc.db).Create(c.Request().Context(), currentSettings)
	}

	settings := json.RawMessage(updateData.MethodSettings)
	currentSettings.MethodSettings = &settings
	currentSettings.WorkoutReply = updateData.WorkoutReply
	currentSettings.WorkoutLike = updateData.WorkoutLike
	currentSettings.FollowRequest = updateData.FollowRequest

	gorm.G[model.UserNotificationSettings](nc.db).Updates(c.Request().Context(), *currentSettings)

	return c.JSON(http.StatusOK, currentSettings)
}
