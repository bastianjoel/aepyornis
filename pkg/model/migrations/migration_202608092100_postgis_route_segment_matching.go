package migrations

import (
	"fmt"

	"github.com/AepyornisNet/aepyornis/pkg/model"
	"gorm.io/gorm"
)

func init() {
	model.RegisterMigration(
		202608092100,
		"Add PostGIS spatial columns and indexes for route segment matching",
		preAutoMigratePostGISMatching,
		postAutoMigratePostGISMatching,
		nil,
		nil,
	)
}

func preAutoMigratePostGISMatching(db *gorm.DB) error {
	// Enable PostGIS extension if available
	_ = db.Exec(`CREATE EXTENSION IF NOT EXISTS postgis`).Error

	// Add geom column to workout_records if missing
	if db.Migrator().HasTable("workout_records") {
		if !db.Migrator().HasColumn("workout_records", "geom") {
			if err := db.Exec(`ALTER TABLE workout_records ADD COLUMN IF NOT EXISTS geom geometry(Point, 4326)`).Error; err != nil {
				// Ignore errors for non-Postgres DBs (e.g. in-memory SQLite test environments)
				_ = err
			}
		}
		_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_workout_records_geom ON workout_records USING GIST (geom)`).Error
	}

	// Add geom column to route_segments if missing
	if db.Migrator().HasTable("route_segments") {
		if !db.Migrator().HasColumn("route_segments", "geom") {
			if err := db.Exec(`ALTER TABLE route_segments ADD COLUMN IF NOT EXISTS geom geometry(Geometry, 4326)`).Error; err != nil {
				_ = err
			}
		}
		_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_route_segments_geom ON route_segments USING GIST (geom)`).Error
	}

	return nil
}

func postAutoMigratePostGISMatching(db *gorm.DB) error {
	if db.Migrator().HasTable("workout_records") && db.Migrator().HasColumn("workout_records", "geom") {
		if err := db.Exec(`UPDATE workout_records SET geom = ST_SetSRID(ST_MakePoint(lng, lat), 4326) WHERE (lat != 0 OR lng != 0) AND geom IS NULL`).Error; err != nil {
			_ = err
		}
	}

	if db.Migrator().HasTable("route_segments") {
		var segments []*model.RouteSegment
		if err := db.Find(&segments).Error; err == nil {
			for _, s := range segments {
				if len(s.Points) > 0 {
					if err := model.UpdateRouteSegmentGeometry(db, s.ID, s.Points); err != nil {
						return fmt.Errorf("updating geometry for route segment %d: %w", s.ID, err)
					}
				}
			}
		}
	}

	return nil
}
