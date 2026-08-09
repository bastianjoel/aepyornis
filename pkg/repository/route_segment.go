package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AepyornisNet/aepyornis/pkg/model"
	"github.com/samber/do/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RouteSegment interface {
	GetByID(id uint64) (*model.RouteSegment, error)
	Count(profileID uint64, isAdmin bool) (int64, error)
	List(limit int, offset int, profileID uint64, isAdmin bool) ([]*model.RouteSegment, error)
	CreateFromContent(profileID uint64, notes string, filename string, content []byte) (*model.RouteSegment, error)
	Save(routeSegment *model.RouteSegment) error
	Delete(routeSegment *model.RouteSegment) error
	CanUserView(rs *model.RouteSegment, profileID uint64, isAdmin bool) bool

	// Likes
	Like(routeSegmentID uint64, profileID uint64) error
	Unlike(routeSegmentID uint64, profileID uint64) error
	HasLiked(routeSegmentID uint64, profileID uint64) (bool, error)
	CountLikes(routeSegmentID uint64) (int64, error)
	GetLikers(routeSegmentID uint64) ([]*model.Profile, error)

	// AP inbox methods for federation
	LikeByActorIRI(routeSegmentID uint64, actorIRI string) error
	UnlikeByActorIRI(routeSegmentID uint64, actorIRI string) error
	ResolveRouteSegmentIDByObjectIRI(objectIRI string) (uint64, error)

	// Matches
	GetMatches(routeSegmentID uint64, sort string, limit int, offset int) ([]*model.RouteSegmentMatch, int64, error)
	GetStats(routeSegmentID uint64) (map[string]interface{}, error)
}

type routeSegmentRepository struct {
	db *gorm.DB
}

func NewRouteSegment(injector do.Injector) (RouteSegment, error) {
	return &routeSegmentRepository{db: do.MustInvoke[*gorm.DB](injector)}, nil
}

func (r *routeSegmentRepository) getFollowingIDs(profileID uint64) []uint64 {
	if profileID == 0 {
		return nil
	}
	var followingIDs []uint64
	r.db.Model(&model.Follower{}).Where("follower_id = ? AND accepted = ?", profileID, true).Pluck("profile_id", &followingIDs)
	return followingIDs
}

func (r *routeSegmentRepository) applyVisibilityFilter(tx *gorm.DB, profileID uint64, isAdmin bool) *gorm.DB {
	if isAdmin {
		return tx
	}

	followingIDs := r.getFollowingIDs(profileID)

	if profileID == 0 {
		return tx.Where("visibility = ?", model.WorkoutVisibilityPublic)
	}

	if len(followingIDs) > 0 {
		return tx.Where("visibility = ? OR profile_id = ? OR (visibility = ? AND profile_id IN (?))",
			model.WorkoutVisibilityPublic, profileID, model.WorkoutVisibilityFollowers, followingIDs)
	}

	return tx.Where("visibility = ? OR profile_id = ?", model.WorkoutVisibilityPublic, profileID)
}

func (r *routeSegmentRepository) CanUserView(rs *model.RouteSegment, profileID uint64, isAdmin bool) bool {
	if rs == nil {
		return false
	}
	if isAdmin || rs.Visibility == model.WorkoutVisibilityPublic || rs.Visibility == "" {
		return true
	}
	if profileID != 0 && rs.ProfileID == profileID {
		return true
	}
	if profileID != 0 && rs.Visibility == model.WorkoutVisibilityFollowers {
		followingIDs := r.getFollowingIDs(profileID)
		for _, id := range followingIDs {
			if id == rs.ProfileID {
				return true
			}
		}
	}
	return false
}

func (r *routeSegmentRepository) GetByID(id uint64) (*model.RouteSegment, error) {
	var routeSegment model.RouteSegment
	if err := r.db.Preload("Profile").First(&routeSegment, id).Error; err != nil {
		return nil, err
	}

	return &routeSegment, nil
}

func (r *routeSegmentRepository) Count(profileID uint64, isAdmin bool) (int64, error) {
	var total int64
	q := r.applyVisibilityFilter(r.db.Model(&model.RouteSegment{}), profileID, isAdmin)
	if err := q.Count(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

func (r *routeSegmentRepository) List(limit int, offset int, profileID uint64, isAdmin bool) ([]*model.RouteSegment, error) {
	var routeSegments []*model.RouteSegment
	q := r.applyVisibilityFilter(r.db.Model(&model.RouteSegment{}), profileID, isAdmin).
		Preload("Profile").
		Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}

	if err := q.Find(&routeSegments).Error; err != nil {
		return nil, err
	}

	return routeSegments, nil
}

func (r *routeSegmentRepository) CreateFromContent(profileID uint64, notes string, filename string, content []byte) (*model.RouteSegment, error) {
	routeSegment, err := model.NewRouteSegment(notes, filename, content)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", model.ErrInvalidData, err)
	}

	if profileID == 0 {
		var firstProfile model.Profile
		if err := r.db.Order("id ASC").First(&firstProfile).Error; err == nil {
			profileID = firstProfile.ID
		}
	}
	routeSegment.ProfileID = profileID

	if err := routeSegment.Create(r.db); err != nil {
		return nil, err
	}

	return routeSegment, nil
}

func (r *routeSegmentRepository) Save(routeSegment *model.RouteSegment) error {
	return routeSegment.Save(r.db)
}

func (r *routeSegmentRepository) Delete(routeSegment *model.RouteSegment) error {
	return routeSegment.Delete(r.db)
}

// routeSegmentStatusID finds or creates the APStatus row for the given route segment,
// following the same pattern used by workoutStatusID in workout_like.go.
func (r *routeSegmentRepository) routeSegmentStatusID(routeSegmentID uint64) (uint64, error) {
	var rs model.RouteSegment
	if err := r.db.First(&rs, routeSegmentID).Error; err != nil {
		return 0, err
	}

	status := &model.APStatus{
		ProfileID:      &rs.ProfileID,
		RouteSegmentID: &routeSegmentID,
		StatusType:     model.APStatusTypeRouteSegment,
		Origin:         "local",
		ActivityID:     fmt.Sprintf("local:route_segment:%d:activity", routeSegmentID),
		ObjectID:       fmt.Sprintf("local:route_segment:%d:object", routeSegmentID),
		Activity:       []byte("{}"),
		Content:        "",
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(status).Error; err != nil {
		return 0, err
	}

	if status.ID == 0 {
		if err := r.db.Where("route_segment_id = ? AND status_type = ?", routeSegmentID, model.APStatusTypeRouteSegment).Take(status).Error; err != nil {
			return 0, err
		}
	}

	return status.ID, nil
}

func (r *routeSegmentRepository) Like(routeSegmentID uint64, profileID uint64) error {
	statusID, err := r.routeSegmentStatusID(routeSegmentID)
	if err != nil {
		return err
	}
	like := &model.APStatusLike{StatusID: statusID, ProfileID: &profileID}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(like).Error
}

func (r *routeSegmentRepository) Unlike(routeSegmentID uint64, profileID uint64) error {
	statusID, err := r.routeSegmentStatusID(routeSegmentID)
	if err != nil {
		return err
	}
	return r.db.Where("status_id = ? AND profile_id = ?", statusID, profileID).Delete(&model.APStatusLike{}).Error
}

func (r *routeSegmentRepository) HasLiked(routeSegmentID uint64, profileID uint64) (bool, error) {
	if profileID == 0 {
		return false, nil
	}
	var count int64
	err := r.db.Table("ap_status_likes").
		Joins("JOIN ap_statuses ON ap_statuses.id = ap_status_likes.status_id").
		Where("ap_statuses.route_segment_id = ? AND ap_status_likes.profile_id = ?", routeSegmentID, profileID).
		Count(&count).Error
	return count > 0, err
}

func (r *routeSegmentRepository) CountLikes(routeSegmentID uint64) (int64, error) {
	var count int64
	err := r.db.Table("ap_status_likes").
		Joins("JOIN ap_statuses ON ap_statuses.id = ap_status_likes.status_id").
		Where("ap_statuses.route_segment_id = ?", routeSegmentID).
		Count(&count).Error
	return count, err
}

func (r *routeSegmentRepository) GetLikers(routeSegmentID uint64) ([]*model.Profile, error) {
	var likes []model.APStatusLike
	err := r.db.Preload("Profile").
		Joins("JOIN ap_statuses ON ap_statuses.id = ap_status_likes.status_id").
		Where("ap_statuses.route_segment_id = ?", routeSegmentID).
		Order("ap_status_likes.created_at DESC").
		Find(&likes).Error
	if err != nil {
		return nil, err
	}

	profiles := make([]*model.Profile, 0, len(likes))
	for _, l := range likes {
		if l.Profile != nil {
			profiles = append(profiles, l.Profile)
		}
	}
	return profiles, nil
}

// ResolveRouteSegmentIDByObjectIRI resolves a route segment ID from an APStatus object IRI.
func (r *routeSegmentRepository) ResolveRouteSegmentIDByObjectIRI(objectIRI string) (uint64, error) {
	var status model.APStatus
	if err := r.db.Where("object_id = ? AND status_type = ?", objectIRI, model.APStatusTypeRouteSegment).First(&status).Error; err != nil {
		return 0, err
	}
	if status.RouteSegmentID == nil {
		return 0, gorm.ErrRecordNotFound
	}
	return *status.RouteSegmentID, nil
}

// LikeByActorIRI likes a route segment by an external actor's IRI (for AP inbox federation).
func (r *routeSegmentRepository) LikeByActorIRI(routeSegmentID uint64, actorIRI string) error {
	if routeSegmentID == 0 || actorIRI == "" {
		return errors.New("route segment id and actor IRI are required")
	}

	profileURL := strings.TrimSpace(actorIRI)
	profile, err := (&model.Profile{URL: &profileURL}).UpsertRemote(r.db)
	if err != nil {
		return err
	}

	statusID, err := r.routeSegmentStatusID(routeSegmentID)
	if err != nil {
		return err
	}

	like := &model.APStatusLike{StatusID: statusID, ProfileID: &profile.ID}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(like).Error
}

// UnlikeByActorIRI removes a like from a route segment by an external actor's IRI.
func (r *routeSegmentRepository) UnlikeByActorIRI(routeSegmentID uint64, actorIRI string) error {
	if routeSegmentID == 0 || actorIRI == "" {
		return errors.New("route segment id and actor IRI are required")
	}

	statusID, err := r.routeSegmentStatusID(routeSegmentID)
	if err != nil {
		return err
	}

	profile := &model.Profile{}
	if err := r.db.Where("url = ?", strings.TrimSpace(actorIRI)).First(profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	return r.db.Where("status_id = ? AND profile_id = ?", statusID, profile.ID).Delete(&model.APStatusLike{}).Error
}

func (r *routeSegmentRepository) GetMatches(routeSegmentID uint64, sort string, limit int, offset int) ([]*model.RouteSegmentMatch, int64, error) {
	var matches []*model.RouteSegmentMatch
	var total int64

	base := r.db.Model(&model.RouteSegmentMatch{}).Where("route_segment_id = ?", routeSegmentID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q := r.db.Preload("Workout.Profile").
		Joins("JOIN workouts ON workouts.id = route_segment_matches.workout_id").
		Where("route_segment_matches.route_segment_id = ?", routeSegmentID)

	switch sort {
	case "newest", "recent":
		q = q.Order("workouts.date DESC, route_segment_matches.duration ASC")
	case "oldest":
		q = q.Order("workouts.date ASC, route_segment_matches.duration ASC")
	case "best", "fastest":
		fallthrough
	default:
		q = q.Order("route_segment_matches.duration ASC, workouts.date DESC")
	}

	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}

	if err := q.Find(&matches).Error; err != nil {
		return nil, 0, err
	}

	return matches, total, nil
}

func (r *routeSegmentRepository) GetStats(routeSegmentID uint64) (map[string]interface{}, error) {
	var totalEfforts int64
	if err := r.db.Model(&model.RouteSegmentMatch{}).Where("route_segment_id = ?", routeSegmentID).Count(&totalEfforts).Error; err != nil {
		return nil, err
	}

	var uniqueAthletes int64
	if err := r.db.Model(&model.RouteSegmentMatch{}).
		Joins("JOIN workouts ON workouts.id = route_segment_matches.workout_id").
		Where("route_segment_matches.route_segment_id = ?", routeSegmentID).
		Select("COUNT(DISTINCT workouts.profile_id)").Scan(&uniqueAthletes).Error; err != nil {
		return nil, err
	}

	var bestMatch model.RouteSegmentMatch
	err := r.db.Preload("Workout.Profile").
		Where("route_segment_id = ?", routeSegmentID).
		Order("duration ASC").
		First(&bestMatch).Error
	var courseRecord *model.RouteSegmentMatch
	if err == nil {
		courseRecord = &bestMatch
	}

	var avgDuration float64
	var avgDistance float64
	if totalEfforts > 0 {
		var result struct {
			AvgDuration float64
			AvgDistance float64
		}
		_ = r.db.Model(&model.RouteSegmentMatch{}).
			Where("route_segment_id = ?", routeSegmentID).
			Select("AVG(duration) as avg_duration, AVG(distance) as avg_distance").
			Scan(&result).Error
		avgDuration = result.AvgDuration
		avgDistance = result.AvgDistance
	}

	return map[string]interface{}{
		"total_efforts":   totalEfforts,
		"unique_athletes": uniqueAthletes,
		"course_record":   courseRecord,
		"avg_duration":    avgDuration,
		"avg_distance":    avgDistance,
	}, nil
}
