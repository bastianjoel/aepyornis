package migrations

import (
	"github.com/AepyornisNet/aepyornis/pkg/model"
	"gorm.io/gorm"
)

func init() {
	model.RegisterMigration(
		202608091600,
		"Add route segment likes and surrogate ID for route segment matches",
		func(db *gorm.DB) error {
			if db.Migrator().HasTable("route_segment_matches") && !db.Migrator().HasColumn("route_segment_matches", "id") {
				_ = db.Exec("ALTER TABLE route_segment_matches DROP CONSTRAINT IF EXISTS route_segment_matches_pkey").Error
			}
			return nil
		},
		nil,
		nil,
		nil,
	)
}
