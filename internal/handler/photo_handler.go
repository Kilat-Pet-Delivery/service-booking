package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Kilat-Pet-Delivery/lib-common/auth"
	"github.com/Kilat-Pet-Delivery/lib-common/middleware"
	"github.com/Kilat-Pet-Delivery/lib-common/response"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/application"
)

// PhotoHandler handles HTTP requests for booking photo operations.
type PhotoHandler struct {
	service *application.PhotoService
}

// NewPhotoHandler creates a new PhotoHandler.
func NewPhotoHandler(service *application.PhotoService) *PhotoHandler {
	return &PhotoHandler{service: service}
}

// RegisterRoutes registers all photo routes.
func (h *PhotoHandler) RegisterRoutes(r *gin.RouterGroup, jwtManager *auth.JWTManager) {
	authMW := middleware.AuthMiddleware(jwtManager)

	photos := r.Group("/api/v1/bookings")
	photos.Use(authMW)
	{
		photos.POST("/:id/photo", middleware.RequireRole(auth.RoleRunner), h.UploadPhoto)
		photos.GET("/:id/photos", h.GetBookingPhotos)
	}
}

// UploadPhoto handles POST /api/v1/bookings/:id/photo.
func (h *PhotoHandler) UploadPhoto(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid booking ID")
		return
	}

	runnerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req application.UploadPhotoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.UploadPhoto(c.Request.Context(), bookingID, runnerID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, result)
}

// GetBookingPhotos handles GET /api/v1/bookings/:id/photos.
func (h *PhotoHandler) GetBookingPhotos(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid booking ID")
		return
	}

	result, err := h.service.GetBookingPhotos(c.Request.Context(), bookingID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}
