package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path"

	"github.com/AepyornisNet/aepyornis/pkg/model"
	"github.com/AepyornisNet/aepyornis/pkg/model/dto"
	"github.com/AepyornisNet/aepyornis/pkg/repository"
	"github.com/AepyornisNet/aepyornis/pkg/worker"
	"github.com/labstack/echo/v4"
	"github.com/samber/do/v2"
	"github.com/spf13/cast"
	"github.com/vgarvardt/gue/v6"
	"gorm.io/gorm"
)

type RouteSegmentController interface {
	GetRouteSegments(c echo.Context) error
	GetRouteSegment(c echo.Context) error
	CreateRouteSegment(c echo.Context) error
	CreateRouteSegmentFromWorkout(c echo.Context) error
	DeleteRouteSegment(c echo.Context) error
	RefreshRouteSegment(c echo.Context) error
	UpdateRouteSegment(c echo.Context) error
	DownloadRouteSegment(c echo.Context) error
	FindRouteSegmentMatches(c echo.Context) error
	GetRouteSegmentMatches(c echo.Context) error
	LikeRouteSegment(c echo.Context) error
	UnlikeRouteSegment(c echo.Context) error
	GetRouteSegmentLikers(c echo.Context) error
}

type routeSegmentController struct {
	client           *gue.Client
	db               *gorm.DB
	logger           *slog.Logger
	routeSegmentRepo repository.RouteSegment
	workoutRepo      repository.Workout
}

func NewRouteSegmentController(injector do.Injector) RouteSegmentController {
	return &routeSegmentController{
		client:           do.MustInvoke[*gue.Client](injector),
		db:               do.MustInvoke[*gorm.DB](injector),
		logger:           do.MustInvoke[*slog.Logger](injector),
		routeSegmentRepo: do.MustInvoke[repository.RouteSegment](injector),
		workoutRepo:      do.MustInvoke[repository.Workout](injector),
	}
}

func (rc *routeSegmentController) getRouteSegment(c echo.Context) (*model.RouteSegment, error) {
	id, err := cast.ToUint64E(c.Param("id"))
	if err != nil {
		return nil, err
	}

	rs, err := rc.routeSegmentRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	return rs, nil
}

// GetRouteSegments returns a paginated list of route segments
// @Summary      List route segments
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Produce      json
// @Param        page      query  int false "Page"
// @Param        per_page  query  int false "Items per page"
// @Success      200  {object}  dto.PaginatedResponse[dto.RouteSegmentResponse]
// @Failure      400  {object}  dto.Response[string]
// @Failure      500  {object}  dto.Response[string]
// @Router       /route-segments [get]
// GetRouteSegments returns a paginated list of route segments
// @Summary      List route segments
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Produce      json
// @Param        page      query  int false "Page"
// @Param        per_page  query  int false "Items per page"
// @Success      200  {object}  dto.PaginatedResponse[dto.RouteSegmentResponse]
// @Failure      400  {object}  dto.Response[string]
// @Failure      500  {object}  dto.Response[string]
// @Router       /route-segments [get]
func (rc *routeSegmentController) GetRouteSegments(c echo.Context) error {
	var pagination dto.PaginationParams
	if err := c.Bind(&pagination); err != nil {
		return renderApiError(c, http.StatusBadRequest, err)
	}
	pagination.SetDefaults()

	var profileID uint64
	var isAdmin bool
	user := currentUser(c)
	if user != nil {
		profileID = user.Profile.ID
		isAdmin = user.Admin
	}

	totalCount, err := rc.routeSegmentRepo.Count(profileID, isAdmin)
	if err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	routeSegments, err := rc.routeSegmentRepo.List(pagination.PerPage, pagination.GetOffset(), profileID, isAdmin)
	if err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	results := dto.NewRouteSegmentsResponse(routeSegments)
	for i, rs := range routeSegments {
		likeCount, _ := rc.routeSegmentRepo.CountLikes(rs.ID)
		hasLiked, _ := rc.routeSegmentRepo.HasLiked(rs.ID, profileID)
		results[i].LikeCount = likeCount
		results[i].HasLiked = hasLiked
		results[i].CanEdit = (profileID != 0 && rs.ProfileID == profileID) || isAdmin
		results[i].CanDelete = (profileID != 0 && rs.ProfileID == profileID) || isAdmin
	}

	resp := dto.PaginatedResponse[dto.RouteSegmentResponse]{
		Results:    results,
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: pagination.CalculateTotalPages(totalCount),
		TotalCount: totalCount,
	}

	return c.JSON(http.StatusOK, resp)
}

// GetRouteSegment returns a single route segment by ID with full details
// @Summary      Get route segment
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Param        id   path  int  true  "Route segment ID"
// @Produce      json
// @Success      200  {object}  dto.Response[dto.RouteSegmentDetailResponse]
// @Failure      404  {object}  dto.Response[string]
// @Router       /route-segments/{id} [get]
func (rc *routeSegmentController) GetRouteSegment(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	var profileID uint64
	var isAdmin bool
	user := currentUser(c)
	if user != nil {
		profileID = user.Profile.ID
		isAdmin = user.Admin
	}

	if !rc.routeSegmentRepo.CanUserView(rs, profileID, isAdmin) {
		return renderApiError(c, http.StatusNotFound, errors.New("route segment not found"))
	}

	detail := dto.NewRouteSegmentDetailResponse(rs)
	likeCount, _ := rc.routeSegmentRepo.CountLikes(rs.ID)
	hasLiked, _ := rc.routeSegmentRepo.HasLiked(rs.ID, profileID)
	detail.LikeCount = likeCount
	detail.HasLiked = hasLiked
	detail.CanEdit = (profileID != 0 && rs.ProfileID == profileID) || isAdmin
	detail.CanDelete = (profileID != 0 && rs.ProfileID == profileID) || isAdmin

	// Compute detailed stats
	if statsMap, err := rc.routeSegmentRepo.GetStats(rs.ID); err == nil {
		statsRes := &dto.RouteSegmentStatsResponse{}
		if te, ok := statsMap["total_efforts"].(int64); ok {
			statsRes.TotalEfforts = te
		}
		if ua, ok := statsMap["unique_athletes"].(int64); ok {
			statsRes.UniqueAthletes = ua
		}
		if ad, ok := statsMap["avg_duration"].(float64); ok {
			statsRes.AvgDuration = ad
		}
		if as, ok := statsMap["avg_distance"].(float64); ok && statsRes.AvgDuration > 0 {
			statsRes.AvgSpeed = as / statsRes.AvgDuration
		}
		if crMatch, ok := statsMap["course_record"].(*model.RouteSegmentMatch); ok && crMatch != nil {
			var crProfileID uint64
			crProfileName := ""
			crWorkoutName := ""
			if crMatch.Workout != nil {
				crWorkoutName = crMatch.Workout.Name
				if crMatch.Workout.Profile != nil {
					crProfileID = crMatch.Workout.Profile.ID
					crProfileName = crMatch.Workout.Profile.DisplayName
				}
			}
			statsRes.CourseRecord = &dto.CourseRecordInfo{
				WorkoutID:   crMatch.WorkoutID,
				WorkoutName: crWorkoutName,
				ProfileID:   crProfileID,
				ProfileName: crProfileName,
				Duration:    int(crMatch.Duration.Seconds()),
				Speed:       crMatch.AverageSpeed(),
			}
		}
		detail.Stats = statsRes
	}

	resp := dto.Response[dto.RouteSegmentDetailResponse]{
		Results: detail,
	}

	return c.JSON(http.StatusOK, resp)
}

// CreateRouteSegment uploads one or more route segment files
// @Summary      Create route segment
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        file   formData  file   true  "GPX file"
// @Param        notes  formData  string false "Notes"
// @Success      201  {object}  dto.Response[dto.RouteSegmentsDetailResponse]
// @Failure      400  {object}  dto.Response[string]
// @Failure      500  {object}  dto.Response[string]
// @Router       /route-segments [post]
func (rc *routeSegmentController) CreateRouteSegment(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return renderApiError(c, http.StatusBadRequest, err)
	}

	files := form.File["file"]
	errMsg := []string{}

	user := currentUser(c)
	var profileID uint64
	if user != nil {
		profileID = user.Profile.ID
	}

	segments := []*dto.RouteSegmentResponse{}
	for _, file := range files {
		content, parseErr := uploadedRouteSegmentFile(file)
		if parseErr != nil {
			errMsg = append(errMsg, parseErr.Error())
			continue
		}

		notes := c.FormValue("notes")

		w, addErr := rc.routeSegmentRepo.CreateFromContent(profileID, notes, file.Filename, content)
		if addErr != nil {
			errMsg = append(errMsg, addErr.Error())
			continue
		}

		if user != nil {
			w.Profile = &user.Profile
		}

		cat := c.FormValue("category")
		if !isValidRouteSegmentCategory(cat) {
			errMsg = append(errMsg, fmt.Sprintf("invalid route segment category: %s", cat))
			continue
		}

		subCat := c.FormValue("sub_category")
		if !isValidRouteSegmentSubCategory(cat, subCat) {
			errMsg = append(errMsg, fmt.Sprintf("invalid route segment sub category: %s", subCat))
			continue
		}

		vis := model.WorkoutVisibility(c.FormValue("visibility"))
		desc := c.FormValue("description")
		diff := model.RouteSegmentDifficulty(c.FormValue("difficulty"))

		if vis == "" {
			vis = model.WorkoutVisibilityPublic
		}
		if vis.IsValid() {
			w.Visibility = vis
		}
		if diff.IsValid() {
			w.Difficulty = diff
		}
		w.Category = cat
		w.SubCategory = subCat
		w.Description = desc

		if err := rc.routeSegmentRepo.Save(w); err != nil {
			rc.logger.Error("Failed to save route segment metadata", "route_segment_id", w.ID, "error", err)
		}

		resp := dto.NewRouteSegmentResponse(w)
		segments = append(segments, &resp)

		if err := worker.EnqueueRouteSegmentUpdate(c.Request().Context(), rc.client, w.ID); err != nil {
			rc.logger.Error("Failed to enqueue route segment update", "route_segment_id", w.ID, "error", err)
		}
	}

	resp := dto.Response[dto.RouteSegmentsDetailResponse]{
		Results: segments,
		Errors:  errMsg,
	}

	return c.JSON(http.StatusCreated, resp)
}

// CreateRouteSegmentFromWorkout creates a route segment from a workout
// @Summary      Create route segment from workout
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Param        id   path  int  true  "Workout ID"
// @Accept       json
// @Produce      json
// @Success      201  {object}  dto.Response[dto.RouteSegmentDetailResponse]
// @Failure      400  {object}  dto.Response[string]
// @Failure      404  {object}  dto.Response[string]
// @Router       /workouts/{id}/route-segment [post]
func (rc *routeSegmentController) CreateRouteSegmentFromWorkout(c echo.Context) error {
	workoutID, err := cast.ToUint64E(c.Param("id"))
	if err != nil {
		return renderApiError(c, http.StatusBadRequest, err)
	}

	workout, err := rc.workoutRepo.GetDetailsByID(workoutID)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	var params model.RoutSegmentCreationParams
	if err := c.Bind(&params); err != nil {
		return renderApiError(c, http.StatusBadRequest, err)
	}

	if !isValidRouteSegmentCategory(params.Category) {
		return renderApiError(c, http.StatusBadRequest, fmt.Errorf("invalid route segment category: %s", params.Category))
	}
	if !isValidRouteSegmentSubCategory(params.Category, params.SubCategory) {
		return renderApiError(c, http.StatusBadRequest, fmt.Errorf("invalid route segment sub category: %s", params.SubCategory))
	}

	content, err := model.RouteSegmentFromPoints(workout, &params)
	if err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	user := currentUser(c)
	var profileID uint64
	if user != nil {
		profileID = user.Profile.ID
	}

	rs, err := rc.routeSegmentRepo.CreateFromContent(profileID, "", params.Filename(), content)
	if err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	if user != nil {
		rs.Profile = &user.Profile
	}
	if params.Visibility != "" {
		rs.Visibility = params.Visibility
	} else {
		rs.Visibility = model.WorkoutVisibilityPublic
	}
	rs.Category = params.Category
	rs.SubCategory = params.SubCategory
	rs.Description = params.Description
	rs.Difficulty = params.Difficulty

	if err := rc.routeSegmentRepo.Save(rs); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	if err := worker.EnqueueRouteSegmentUpdate(c.Request().Context(), rc.client, rs.ID); err != nil {
		rc.logger.Error("Failed to enqueue route segment update", "route_segment_id", rs.ID, "error", err)
	}

	resp := dto.Response[dto.RouteSegmentDetailResponse]{
		Results: dto.NewRouteSegmentDetailResponse(rs),
	}

	return c.JSON(http.StatusCreated, resp)
}

// DeleteRouteSegment deletes a route segment
// @Summary      Delete route segment
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Param        id   path  int  true  "Route segment ID"
// @Produce      json
// @Success      200  {object}  dto.Response[map[string]string]
// @Failure      404  {object}  dto.Response[string]
// @Failure      500  {object}  dto.Response[string]
// @Router       /route-segments/{id} [delete]
func canModifyRouteSegment(c echo.Context, rs *model.RouteSegment) bool {
	user := currentUser(c)
	if user == nil {
		return false
	}
	if user.Admin {
		return true
	}
	return rs.ProfileID != 0 && rs.ProfileID == user.Profile.ID
}

// DeleteRouteSegment deletes a route segment
// @Summary      Delete route segment
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Param        id   path  int  true  "Route segment ID"
// @Produce      json
// @Success      200  {object}  dto.Response[map[string]string]
// @Failure      403  {object}  dto.Response[string]
// @Failure      404  {object}  dto.Response[string]
// @Failure      500  {object}  dto.Response[string]
// @Router       /route-segments/{id} [delete]
func (rc *routeSegmentController) DeleteRouteSegment(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	if !canModifyRouteSegment(c, rs) {
		return renderApiError(c, http.StatusForbidden, errors.New("forbidden: you do not have permission to delete this route segment"))
	}

	if err := rc.routeSegmentRepo.Delete(rs); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	resp := dto.Response[map[string]string]{
		Results: map[string]string{"message": "Route segment deleted successfully"},
	}

	return c.JSON(http.StatusOK, resp)
}

// RefreshRouteSegment marks a route segment for refresh
// @Summary      Refresh route segment
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Param        id   path  int  true  "Route segment ID"
// @Produce      json
// @Success      200  {object}  dto.Response[map[string]string]
// @Failure      403  {object}  dto.Response[string]
// @Failure      404  {object}  dto.Response[string]
// @Failure      500  {object}  dto.Response[string]
// @Router       /route-segments/{id}/refresh [post]
func (rc *routeSegmentController) RefreshRouteSegment(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	if !canModifyRouteSegment(c, rs) {
		return renderApiError(c, http.StatusForbidden, errors.New("forbidden: you do not have permission to refresh this route segment"))
	}

	if err := rs.UpdateFromContent(); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	if err := rc.routeSegmentRepo.Save(rs); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	resp := dto.Response[map[string]string]{
		Results: map[string]string{"message": "Route segment refreshed successfully"},
	}

	return c.JSON(http.StatusOK, resp)
}

// UpdateRouteSegment updates a route segment
// @Summary      Update route segment
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Param        id   path  int  true  "Route segment ID"
// @Accept       json
// @Produce      json
// @Success      200  {object}  dto.Response[dto.RouteSegmentDetailResponse]
// @Failure      400  {object}  dto.Response[string]
// @Failure      403  {object}  dto.Response[string]
// @Failure      404  {object}  dto.Response[string]
// @Failure      500  {object}  dto.Response[string]
// @Router       /route-segments/{id} [put]
func (rc *routeSegmentController) UpdateRouteSegment(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	if !canModifyRouteSegment(c, rs) {
		return renderApiError(c, http.StatusForbidden, errors.New("forbidden: you do not have permission to update this route segment"))
	}

	type updateParams struct {
		Name          string                       `json:"name"`
		Notes         string                       `json:"notes"`
		Category      string                       `json:"category"`
		SubCategory   string                       `json:"sub_category"`
		Visibility    model.WorkoutVisibility      `json:"visibility"`
		Description   string                       `json:"description"`
		Difficulty    model.RouteSegmentDifficulty `json:"difficulty"`
		Bidirectional bool                         `json:"bidirectional"`
		Circular      bool                         `json:"circular"`
	}

	var params updateParams
	if err := c.Bind(&params); err != nil {
		return renderApiError(c, http.StatusBadRequest, err)
	}

	if len(params.Name) == 0 {
		return renderApiError(c, http.StatusBadRequest, errors.New("route segment name is required"))
	}
	if params.Visibility != "" && !params.Visibility.IsValid() {
		return renderApiError(c, http.StatusBadRequest, errors.New("invalid route segment visibility"))
	}
	if params.Difficulty != "" && !params.Difficulty.IsValid() {
		return renderApiError(c, http.StatusBadRequest, errors.New("invalid route segment difficulty"))
	}
	if !isValidRouteSegmentCategory(params.Category) {
		return renderApiError(c, http.StatusBadRequest, fmt.Errorf("invalid route segment category: %s", params.Category))
	}
	if !isValidRouteSegmentSubCategory(params.Category, params.SubCategory) {
		return renderApiError(c, http.StatusBadRequest, fmt.Errorf("invalid route segment sub category: %s", params.SubCategory))
	}

	rs.Name = params.Name
	rs.Notes = params.Notes
	rs.Category = params.Category
	rs.SubCategory = params.SubCategory
	if params.Visibility != "" {
		rs.Visibility = params.Visibility
	}
	rs.Description = params.Description
	rs.Difficulty = params.Difficulty
	rs.Bidirectional = params.Bidirectional
	rs.Circular = params.Circular
	rs.Dirty = true

	if err := rs.Save(rc.db); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	if err := worker.EnqueueRouteSegmentUpdate(c.Request().Context(), rc.client, rs.ID); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	resp := dto.Response[dto.RouteSegmentDetailResponse]{
		Results: dto.NewRouteSegmentDetailResponse(rs),
	}

	return c.JSON(http.StatusOK, resp)
}

// DownloadRouteSegment downloads the original route segment file
// @Summary      Download route segment file
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Param        id   path  int  true  "Route segment ID"
// @Produce      octet-stream
// @Success      200  {string}  string  "binary GPX content"
// @Failure      404  {object}  dto.Response[string]
// @Router       /route-segments/{id}/download [get]
func (rc *routeSegmentController) DownloadRouteSegment(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	basename := path.Base(rs.Filename)
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+basename+"\"")

	return c.Stream(http.StatusOK, "application/binary", bytes.NewReader(rs.Content))
}

// FindRouteSegmentMatches finds matching workouts for a route segment
// @Summary      Find matching workouts
// @Tags         route-segments
// @Security     ApiKeyAuth
// @Security     ApiKeyQuery
// @Security     CookieAuth
// @Param        id   path  int  true  "Route segment ID"
// @Produce      json
// @Success      200  {object}  dto.Response[map[string]string]
// @Failure      404  {object}  dto.Response[string]
// @Failure      500  {object}  dto.Response[string]
// @Router       /route-segments/{id}/matches [post]
func (rc *routeSegmentController) FindRouteSegmentMatches(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	rs.Dirty = true
	if err := rs.Save(rc.db); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	if err := worker.EnqueueRouteSegmentUpdate(c.Request().Context(), rc.client, rs.ID); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	resp := dto.Response[map[string]string]{
		Results: map[string]string{"message": "Finding matches in background"},
	}

	return c.JSON(http.StatusOK, resp)
}

func (rc *routeSegmentController) GetRouteSegmentMatches(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	var profileID uint64
	var isAdmin bool
	user := currentUser(c)
	if user != nil {
		profileID = user.Profile.ID
		isAdmin = user.Admin
	}

	if !rc.routeSegmentRepo.CanUserView(rs, profileID, isAdmin) {
		return renderApiError(c, http.StatusNotFound, errors.New("route segment not found"))
	}

	var pagination dto.PaginationParams
	if err := c.Bind(&pagination); err != nil {
		return renderApiError(c, http.StatusBadRequest, err)
	}
	pagination.SetDefaults()

	sort := c.QueryParam("sort")
	matches, totalCount, err := rc.routeSegmentRepo.GetMatches(rs.ID, sort, pagination.PerPage, pagination.GetOffset())
	if err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	results := make([]dto.RouteSegmentMatch, len(matches))
	for i, m := range matches {
		results[i] = dto.NewRouteSegmentMatchResponse(m)
	}

	resp := dto.PaginatedResponse[dto.RouteSegmentMatch]{
		Results:    results,
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: pagination.CalculateTotalPages(totalCount),
		TotalCount: totalCount,
	}

	return c.JSON(http.StatusOK, resp)
}

func (rc *routeSegmentController) LikeRouteSegment(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	user := currentUser(c)
	if user == nil {
		return renderApiError(c, http.StatusUnauthorized, errors.New("unauthorized"))
	}

	if err := rc.routeSegmentRepo.Like(rs.ID, user.Profile.ID); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	count, _ := rc.routeSegmentRepo.CountLikes(rs.ID)
	return c.JSON(http.StatusOK, dto.Response[map[string]interface{}]{
		Results: map[string]interface{}{"liked": true, "like_count": count},
	})
}

func (rc *routeSegmentController) UnlikeRouteSegment(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	user := currentUser(c)
	if user == nil {
		return renderApiError(c, http.StatusUnauthorized, errors.New("unauthorized"))
	}

	if err := rc.routeSegmentRepo.Unlike(rs.ID, user.Profile.ID); err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	count, _ := rc.routeSegmentRepo.CountLikes(rs.ID)
	return c.JSON(http.StatusOK, dto.Response[map[string]interface{}]{
		Results: map[string]interface{}{"liked": false, "like_count": count},
	})
}

func (rc *routeSegmentController) GetRouteSegmentLikers(c echo.Context) error {
	rs, err := rc.getRouteSegment(c)
	if err != nil {
		return renderApiError(c, http.StatusNotFound, err)
	}

	likers, err := rc.routeSegmentRepo.GetLikers(rs.ID)
	if err != nil {
		return renderApiError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, dto.Response[[]*model.Profile]{
		Results: likers,
	})
}

func uploadedRouteSegmentFile(file *multipart.FileHeader) ([]byte, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	return content, nil
}

func isValidRouteSegmentCategory(cat string) bool {
	if cat == "" {
		return true
	}
	wt, valid := model.ParseWorkoutType(cat)
	return valid && wt != model.WorkoutTypeAll && wt != model.WorkoutTypeUnknown
}

func isValidRouteSegmentSubCategory(cat string, subCat string) bool {
	if subCat == "" {
		return true
	}
	return model.IsValidWorkoutSubTypeForCategory(model.WorkoutType(cat), subCat)
}
