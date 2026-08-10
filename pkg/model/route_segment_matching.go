package model

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MaxDeltaMeter is the maximum distance in meters that a point can be away from
// the route segment
const MaxDeltaMeter = 20.0

// MaxTotalDistanceFraction is the maximum percentage of the total distance of
// the route segment that can be exceeded by the total distance matching part of
// the route (1.0 = 100%)
const MaxTotalDistanceFraction = 0.9

// RouteSegmentMatch is a match between a route segment and a workout
type RouteSegmentMatch struct {
	ID           uint64        `gorm:"primaryKey;autoIncrement" json:"id"`
	Workout      *Workout      `json:"workout"`
	RouteSegment *RouteSegment `json:"routeSegment"`

	first, last WorkoutRecord // The first and last point of the route
	end         WorkoutRecord // The last point of the workout

	RouteSegmentID uint64        `gorm:"not null;index" json:"routeSegmentID"` // The ID of the route segment
	WorkoutID      uint64        `gorm:"not null;index" json:"workoutID"`      // The ID of the workout
	FirstID        int           `json:"firstID"`                          // The index of the first point of the route
	LastID         int           `json:"lastID"`                           // The index of the last point of the route
	Distance       float64       `json:"distance"`                         // The total distance of the route segment for this workout
	Duration       time.Duration `json:"duration"`                         // The total duration of the route segment for this workout
}

func (rsm *RouteSegmentMatch) AverageSpeed() float64 {
	if rsm.Duration.Seconds() == 0 {
		return 0
	}
	return rsm.Distance / rsm.Duration.Seconds()
}

// NewRouteSegmentMatch will create a new route segment match from a workout and
// the first and last point of the route along the route segment
func (rs *RouteSegment) NewRouteSegmentMatch(workout *Workout, p, last int) *RouteSegmentMatch {
	rsm := &RouteSegmentMatch{
		Workout:      workout,
		RouteSegment: rs,
		FirstID:      p,
		LastID:       last,
	}

	rsm.calculate()

	return rsm
}

// IsBetterThan returns true if the new route segment match is better than the
// current one
func (rsm *RouteSegmentMatch) IsBetterThan(current *RouteSegmentMatch) bool {
	return current == nil || rsm.Distance < current.Distance
}

// MatchesDistance returns true if the distance of the route segment match is
// within MaxTotalDistancePercentage of the distance of the current route
// segment
func (rsm *RouteSegmentMatch) MatchesDistance(distance float64) bool {
	if distance == 0 {
		return true
	}
	return math.Abs(rsm.Distance/distance) > MaxTotalDistanceFraction
}

// calculate will calculate the total distance and duration of the route
// segment, and the total number of points of this workout along the route
// segment
func (rsm *RouteSegmentMatch) calculate() {
	rsm.RouteSegmentID = rsm.RouteSegment.ID
	rsm.WorkoutID = rsm.Workout.ID
	rsm.first = rsm.Workout.Records[rsm.FirstID]
	rsm.last = rsm.Workout.Records[rsm.LastID]
	rsm.end = rsm.Workout.Records[len(rsm.Workout.Records)-1]

	if rsm.FirstID <= rsm.LastID {
		rsm.Distance = rsm.last.TotalDistance - rsm.first.TotalDistance
		rsm.Duration = rsm.last.TotalDuration - rsm.first.TotalDuration
	} else {
		rsm.Distance = rsm.last.TotalDistance + rsm.end.TotalDistance - rsm.first.TotalDistance
		rsm.Duration = rsm.last.TotalDuration + rsm.end.TotalDuration - rsm.first.TotalDuration
	}
}

// FindMatches will find all workouts that match the current route segment
func (rs *RouteSegment) FindMatches(workouts []*Workout) []*RouteSegmentMatch {
	if len(rs.Points) == 0 {
		return nil
	}

	var result []*RouteSegmentMatch

	for _, w := range workouts {
		if m := rs.Match(w); m != nil {
			result = append(result, m)
		}
	}

	return result
}

// Match will find the best match (if any) of the route segment in the workout
func (rs *RouteSegment) Match(workout *Workout) *RouteSegmentMatch {
	if !workout.Type.IsLocation() {
		return nil
	}

	if !workout.HasTracks() {
		return nil
	}

	sp := rs.StartingPoints(workout.Records)
	if len(sp) == 0 {
		return nil
	}

	var bestMatch *RouteSegmentMatch

	for _, p := range sp {
		if last, ok := rs.MatchSegment(workout, p, true); ok {
			rsm := rs.NewRouteSegmentMatch(workout, p, last)
			if rsm.MatchesDistance(rs.TotalDistance) && rsm.IsBetterThan(bestMatch) {
				bestMatch = rsm
			}
		}

		if !rs.Bidirectional {
			continue
		}

		if last, ok := rs.MatchSegment(workout, p, false); ok {
			rsm := rs.NewRouteSegmentMatch(workout, p, last)
			if rsm.MatchesDistance(rs.TotalDistance) && rsm.IsBetterThan(bestMatch) {
				bestMatch = rsm
			}
		}
	}

	return bestMatch
}

// MatchSegment starts at a point and continues the workout track while it finds
// each next point of the route segment.
func (rs *RouteSegment) MatchSegment(workout *Workout, start int, forward bool) (int, bool) {
	workoutLength := len(workout.Records)
	segmentLength := len(rs.Points)

	cur := 0
	if !forward {
		cur = segmentLength - 1
	}

	for i := range workoutLength {
		index := (start + i) % workoutLength

		d := rs.Points[cur].DistanceTo(&workout.Records[index])
		if d > MaxDeltaMeter {
			continue
		}

		if forward {
			cur++

			if cur == segmentLength {
				return index, true
			}
		} else {
			cur--

			if cur == 0 {
				return index, true
			}
		}

		if !rs.Circular && index < start {
			break
		}
	}

	return 0, false
}

// StartingPoints finds all points that are closer than MaxDeltaMeter to the
// segment's starting point
func (rs *RouteSegment) StartingPoints(points []WorkoutRecord) []int {
	var r []int

	start := rs.Points[0]

	for i, p := range points {
		d := start.DistanceTo(&p)
		if d < MaxDeltaMeter {
			r = append(r, i)
		}
	}

	return r
}

// FindMatches will find all workouts that match the current route segment
func (w *Workout) FindMatches(routeSegments []*RouteSegment) []*RouteSegmentMatch {
	if !w.HasTracks() {
		return nil
	}

	var result []*RouteSegmentMatch

	for _, rs := range routeSegments {
		if m := rs.Match(w); m != nil {
			result = append(result, m)
		}
	}

	return result
}

// PostGISMatchResult holds the raw match data returned by the PostGIS spatial join query
type PostGISMatchResult struct {
	RouteSegmentID   uint64  `gorm:"column:route_segment_id"`
	WorkoutID        uint64  `gorm:"column:workout_id"`
	StartIndex       int     `gorm:"column:start_index"`
	EndIndex         int     `gorm:"column:end_index"`
	MatchedPoints    int     `gorm:"column:matched_points"`
	Distance         float64 `gorm:"column:distance"`
	DurationNS       int64   `gorm:"column:duration_ns"`
	SegmentTotalDist float64 `gorm:"column:segment_total_distance"`
}

// FindPostGISRouteSegmentMatches executes a native PostGIS spatial join to match route segments against workouts.
// - If routeSegmentID > 0, matches for that specific segment are queried.
// - If workoutID > 0, matches for that specific workout are queried.
// - Points with 0/0 lat/lng (no GPS fix) are filtered out via (w.lat != 0 OR w.lng != 0).
func FindPostGISRouteSegmentMatches(db *gorm.DB, routeSegmentID uint64, workoutID uint64) ([]*RouteSegmentMatch, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}

	query := `
	WITH matched_points AS (
		SELECT 
			s.id AS route_segment_id,
			w.workout_id AS workout_id,
			w.sort_order AS point_index,
			LAG(w.sort_order) OVER (PARTITION BY s.id, w.workout_id ORDER BY w.sort_order) AS prev_index
		FROM 
			route_segments s
		JOIN 
			workout_records w
		ON 
			ST_DWithin(s.geom::geography, w.geom::geography, 50)
		WHERE 
			(w.lat != 0 OR w.lng != 0)
			AND w.geom IS NOT NULL
			AND s.geom IS NOT NULL
			AND (? = 0 OR s.id = ?)
			AND (? = 0 OR w.workout_id = ?)
	),
	matched_groups AS (
		SELECT 
			route_segment_id,
			workout_id,
			point_index,
			SUM(CASE WHEN prev_index IS NULL OR point_index - prev_index > 10 THEN 1 ELSE 0 END) 
				OVER (PARTITION BY route_segment_id, workout_id ORDER BY point_index) AS grp
		FROM 
			matched_points
	),
	match_summary AS (
		SELECT 
			mg.route_segment_id,
			mg.workout_id,
			MIN(mg.point_index) AS start_index,
			MAX(mg.point_index) AS end_index,
			COUNT(mg.point_index) AS matched_points
		FROM 
			matched_groups mg
		GROUP BY 
			mg.route_segment_id,
			mg.workout_id,
			mg.grp
		HAVING 
			COUNT(mg.point_index) >= 2
	)
	SELECT 
		ms.route_segment_id,
		ms.workout_id,
		ms.start_index,
		ms.end_index,
		ms.matched_points,
		COALESCE(ABS(w_end.total_distance - w_start.total_distance), 0) AS distance,
		COALESCE(ABS(w_end.total_duration - w_start.total_duration), 0) AS duration_ns,
		COALESCE(s.total_distance, 0) AS segment_total_distance
	FROM 
		match_summary ms
	JOIN 
		route_segments s ON s.id = ms.route_segment_id
	JOIN 
		workout_records w_start ON w_start.workout_id = ms.workout_id AND w_start.sort_order = ms.start_index
	JOIN 
		workout_records w_end ON w_end.workout_id = ms.workout_id AND w_end.sort_order = ms.end_index
	ORDER BY 
		ms.workout_id, 
		ms.start_index;
	`

	var rawResults []PostGISMatchResult
	err := db.Raw(query, routeSegmentID, routeSegmentID, workoutID, workoutID).Scan(&rawResults).Error
	if err != nil {
		return nil, err
	}

	matches := make([]*RouteSegmentMatch, 0, len(rawResults))
	for _, res := range rawResults {
		dist := res.Distance
		if res.SegmentTotalDist > 0 && math.Abs(dist/res.SegmentTotalDist) <= MaxTotalDistanceFraction {
			continue
		}

		m := &RouteSegmentMatch{
			RouteSegmentID: res.RouteSegmentID,
			WorkoutID:      res.WorkoutID,
			FirstID:        res.StartIndex,
			LastID:         res.EndIndex,
			Distance:       dist,
			Duration:       time.Duration(res.DurationNS),
		}
		matches = append(matches, m)
	}

	return matches, nil
}

// UpdateRouteSegmentGeometry updates the PostGIS geometry column for a route segment.
func UpdateRouteSegmentGeometry(db *gorm.DB, segmentID uint64, points []WorkoutRecord) error {
	if db == nil || segmentID == 0 {
		return nil
	}

	var valid []WorkoutRecord
	for _, p := range points {
		if p.Lat != 0 || p.Lng != 0 {
			valid = append(valid, p)
		}
	}

	if len(valid) == 0 {
		return db.Exec(`UPDATE route_segments SET geom = NULL WHERE id = ?`, segmentID).Error
	}

	if len(valid) == 1 {
		wkt := fmt.Sprintf("POINT(%.7f %.7f)", valid[0].Lng, valid[0].Lat)
		return db.Exec(`UPDATE route_segments SET geom = ST_SetSRID(ST_GeomFromText(?, 4326), 4326) WHERE id = ?`, wkt, segmentID).Error
	}

	var sb strings.Builder
	sb.WriteString("LINESTRING(")
	for i, p := range valid {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%.7f %.7f", p.Lng, p.Lat))
	}
	sb.WriteString(")")

	return db.Exec(`UPDATE route_segments SET geom = ST_SetSRID(ST_GeomFromText(?, 4326), 4326) WHERE id = ?`, sb.String(), segmentID).Error
}
