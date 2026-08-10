package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/AepyornisNet/aepyornis/pkg/model"
	"github.com/AepyornisNet/aepyornis/pkg/repository"
	"github.com/vgarvardt/gue/v6"
	"gorm.io/gorm"
)

const JobUpdateRouteSegment = "update_route_segment"

const workerWorkoutsBatchSize = 10

// EnqueueRouteSegmentUpdate enqueues a job to re-match the given route segment.
// Call this wherever a route segment is created or marked dirty.
func EnqueueRouteSegmentUpdate(ctx context.Context, client *gue.Client, segmentID uint64) error {
	return enqueueJob(ctx, client, MainQueue, JobUpdateRouteSegment, idArgs{ID: segmentID})
}

func makeUpdateRouteSegmentHandler(db *gorm.DB, logger *slog.Logger, routeSegmentRepo repository.RouteSegment) gue.WorkFunc {
	return func(ctx context.Context, j *gue.Job) error {
		var args idArgs
		if err := json.Unmarshal(j.Args, &args); err != nil {
			return fmt.Errorf("update_route_segment: unmarshal args: %w", err)
		}

		l := logger.With("route_segment_id", args.ID)

		rs, err := routeSegmentRepo.GetByID(args.ID)
		if err != nil {
			return fmt.Errorf("update_route_segment: get route segment %d: %w", args.ID, err)
		}

		if !rs.Dirty {
			return nil
		}

		l.Info("Updating route segment")

		return rematchRouteSegmentToWorkouts(db, rs, l)
	}
}

func rematchRouteSegmentToWorkouts(db *gorm.DB, rs *model.RouteSegment, l *slog.Logger) error {
	if err := model.UpdateRouteSegmentGeometry(db, rs.ID, rs.Points); err != nil {
		l.Warn("Failed to update route segment geometry", "error", err)
	}

	matches, err := model.FindPostGISRouteSegmentMatches(db, rs.ID, 0)
	if err != nil {
		l.Warn("PostGIS matching query unavailable, falling back to batch matching", "error", err)

		rs.RouteSegmentMatches = []*model.RouteSegmentMatch{}
		var workoutsBatch []*model.Workout
		qw := model.PreloadWorkoutDetails(db).Model(&model.Workout{}).
			FindInBatches(&workoutsBatch, workerWorkoutsBatchSize, func(wtx *gorm.DB, batchNo int) error {
				newMatches := rs.FindMatches(workoutsBatch)
				rs.RouteSegmentMatches = append(rs.RouteSegmentMatches, newMatches...)
				return nil
			})
		if qw.Error != nil {
			return fmt.Errorf("error in batch processing of route segment matching: %w", qw.Error)
		}
	} else {
		rs.RouteSegmentMatches = matches
	}

	l.With("route_segment_id", rs.ID).
		With("matches_found", len(rs.RouteSegmentMatches)).
		Info("Route segment matching completed")

	rs.Dirty = false

	if err := rs.Save(db); err != nil {
		return fmt.Errorf("error saving route segment: %w", err)
	}

	return nil
}
