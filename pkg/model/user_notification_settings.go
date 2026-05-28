package model

import "encoding/json"

type UserNotificationSettings struct {
	Model

	UserID uint64 `gorm:"not null" json:"-"`          // The ID of the user the notification is sent to
	User   *User  `gorm:"foreignKey:UserID" json:"-"` // The user this notification is sent to

	Method         string           `json:"method"`
	MethodSettings *json.RawMessage `json:"method_settings,omitempty"`

	FollowRequest bool `gorm:"default:true" json:"follow_request"`
	WorkoutLike   bool `gorm:"default:true" json:"workout_like"`
	WorkoutReply  bool `gorm:"default:true" json:"workout_reply"`
}
