package model

import "time"

type HammerheadConnection struct {
	Model
	UserID           uint64    `gorm:"not null;uniqueIndex" json:"user_id"`
	User             User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	HammerheadUserID string    `gorm:"not null;uniqueIndex;type:varchar(64)" json:"hammerhead_user_id"`
	AccessToken      string    `gorm:"not null;type:text" json:"-"`
	RefreshToken     string    `gorm:"not null;type:text" json:"-"`
	Scope            string    `gorm:"not null;type:varchar(256)" json:"-"`
	ExpiresAt        time.Time `gorm:"not null" json:"-"`
}

func (c *HammerheadConnection) AccessTokenExpired() bool {
	if c == nil {
		return true
	}

	return time.Now().After(c.ExpiresAt)
}
