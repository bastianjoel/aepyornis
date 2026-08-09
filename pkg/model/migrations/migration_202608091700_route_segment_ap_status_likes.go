package migrations

import (
	"fmt"

	"github.com/AepyornisNet/aepyornis/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	model.RegisterMigration(
		202608091700,
		"Migrate route segment likes to ap_status_likes",
		preAutoMigrateRouteSegmentAPStatusLikes,
		nil,
		postAutoMigrateRouteSegmentAPStatusLikes,
		nil,
	)
}

func preAutoMigrateRouteSegmentAPStatusLikes(db *gorm.DB) error {
	if !db.Migrator().HasTable("ap_statuses") {
		return nil
	}

	// Add route_segment_id column to ap_statuses before GORM AutoMigrate processes the updated struct
	if !db.Migrator().HasColumn("ap_statuses", "route_segment_id") {
		if err := db.Exec(`ALTER TABLE ap_statuses ADD COLUMN IF NOT EXISTS route_segment_id BIGINT REFERENCES route_segments(id) ON DELETE SET NULL`).Error; err != nil {
			return fmt.Errorf("adding route_segment_id to ap_statuses: %w", err)
		}
		_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_ap_statuses_route_segment_id ON ap_statuses(route_segment_id)`)
	}

	return nil
}

func postAutoMigrateRouteSegmentAPStatusLikes(db *gorm.DB) error {
	if !db.Migrator().HasTable("route_segment_likes") {
		return nil
	}

	// Migrate existing route_segment_likes into ap_status_likes
	type oldLike struct {
		ID             uint64
		RouteSegmentID uint64
		ProfileID      uint64
		CreatedAt      string
	}

	var oldLikes []oldLike
	if err := db.Table("route_segment_likes").Find(&oldLikes).Error; err != nil {
		return fmt.Errorf("reading route_segment_likes: %w", err)
	}

	for _, ol := range oldLikes {
		// Look up or create the APStatus for this route segment
		var rs model.RouteSegment
		if err := db.First(&rs, ol.RouteSegmentID).Error; err != nil {
			continue // segment deleted, skip
		}

		status := &model.APStatus{
			ProfileID:      &rs.ProfileID,
			RouteSegmentID: &ol.RouteSegmentID,
			StatusType:     model.APStatusTypeRouteSegment,
			Origin:         "local",
			ActivityID:     fmt.Sprintf("local:route_segment:%d:activity", ol.RouteSegmentID),
			ObjectID:       fmt.Sprintf("local:route_segment:%d:object", ol.RouteSegmentID),
			Activity:       []byte("{}"),
			Content:        "",
		}

		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(status).Error; err != nil {
			return fmt.Errorf("creating ap_status for route segment %d: %w", ol.RouteSegmentID, err)
		}

		if status.ID == 0 {
			if err := db.Where("route_segment_id = ? AND status_type = ?", ol.RouteSegmentID, model.APStatusTypeRouteSegment).Take(status).Error; err != nil {
				continue
			}
		}

		profileID := ol.ProfileID
		like := &model.APStatusLike{
			StatusID:  status.ID,
			ProfileID: &profileID,
		}
		_ = db.Clauses(clause.OnConflict{DoNothing: true}).Create(like).Error
	}

	// Drop the old route_segment_likes table
	if err := db.Exec(`DROP TABLE IF EXISTS route_segment_likes`).Error; err != nil {
		return fmt.Errorf("dropping route_segment_likes: %w", err)
	}

	return nil
}
