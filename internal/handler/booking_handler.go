package handler

import (
	"net/http"
	"strconv"

	"github.com/Kilat-Pet-Delivery/lib-common/auth"
	"github.com/Kilat-Pet-Delivery/lib-common/middleware"
	"github.com/Kilat-Pet-Delivery/lib-common/response"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/application"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BookingHandler handles HTTP requests for booking operations.
type BookingHandler struct {
	service *application.BookingService
}

// NewBookingHandler creates a new BookingHandler.
func NewBookingHandler(service *application.BookingService) *BookingHandler {
	return &BookingHandler{service: service}
}

// RegisterRoutes registers all booking routes on the given router group.
func (h *BookingHandler) RegisterRoutes(r *gin.RouterGroup, jwtManager *auth.JWTManager) {
	authMW := middleware.AuthMiddleware(jwtManager)

	bookings := r.Group("/api/v1/bookings")
	bookings.Use(authMW)
	{
		bookings.POST("", middleware.RequireRole(auth.RoleOwner), h.CreateBooking)
		bookings.GET("", h.ListBookings)
		bookings.GET("/:id", h.GetBooking)
		bookings.POST("/:id/accept", middleware.RequireRole(auth.RoleRunner), h.AcceptBooking)
		bookings.POST("/:id/pickup", middleware.RequireRole(auth.RoleRunner), h.StartDelivery)
		bookings.POST("/:id/deliver", middleware.RequireRole(auth.RoleRunner), h.ConfirmDelivery)
		bookings.POST("/:id/confirm", middleware.RequireRole(auth.RoleOwner), h.ConfirmDeliveryByOwner)
		bookings.POST("/:id/cancel", h.CancelBooking)
		bookings.POST("/:id/rebook", middleware.RequireRole(auth.RoleOwner), h.RebookBooking)
	}
}

// CreateBooking handles POST /api/v1/bookings.
func (h *BookingHandler) CreateBooking(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req application.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.CreateBooking(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, result)
}

// ListBookings handles GET /api/v1/bookings. Filters by role (owner sees own, runner sees assigned).
func (h *BookingHandler) ListBookings(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	role, ok := middleware.GetUserRole(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	page, limit := parsePagination(c)

	switch role {
	case auth.RoleOwner:
		result, err := h.service.GetOwnerBookings(c.Request.Context(), userID, page, limit)
		if err != nil {
			response.Error(c, err)
			return
		}
		response.Paginated(c, result.Items, result.Total, result.Page, result.Limit)

	case auth.RoleRunner:
		result, err := h.service.GetRunnerBookings(c.Request.Context(), userID, page, limit)
		if err != nil {
			response.Error(c, err)
			return
		}
		response.Paginated(c, result.Items, result.Total, result.Page, result.Limit)

	default:
		// Admin or other roles can see owner bookings by default
		result, err := h.service.GetOwnerBookings(c.Request.Context(), userID, page, limit)
		if err != nil {
			response.Error(c, err)
			return
		}
		response.Paginated(c, result.Items, result.Total, result.Page, result.Limit)
	}
}

// GetBooking handles GET /api/v1/bookings/:id.
func (h *BookingHandler) GetBooking(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid booking ID")
		return
	}

	result, err := h.service.GetBooking(c.Request.Context(), bookingID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// AcceptBooking handles POST /api/v1/bookings/:id/accept.
func (h *BookingHandler) AcceptBooking(c *gin.Context) {
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

	result, err := h.service.AcceptBooking(c.Request.Context(), bookingID, runnerID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// StartDelivery handles POST /api/v1/bookings/:id/pickup.
func (h *BookingHandler) StartDelivery(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid booking ID")
		return
	}

	result, err := h.service.StartDelivery(c.Request.Context(), bookingID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// ConfirmDelivery handles POST /api/v1/bookings/:id/deliver (runner marks delivered).
func (h *BookingHandler) ConfirmDelivery(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid booking ID")
		return
	}

	result, err := h.service.ConfirmDelivery(c.Request.Context(), bookingID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// ConfirmDeliveryByOwner handles POST /api/v1/bookings/:id/confirm (owner confirms delivery).
func (h *BookingHandler) ConfirmDeliveryByOwner(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid booking ID")
		return
	}

	result, err := h.service.CompleteBooking(c.Request.Context(), bookingID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// CancelBooking handles POST /api/v1/bookings/:id/cancel.
func (h *BookingHandler) CancelBooking(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid booking ID")
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)

	result, err := h.service.CancelBooking(c.Request.Context(), bookingID, userID, body.Reason)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// RebookBooking handles POST /api/v1/bookings/:id/rebook.
func (h *BookingHandler) RebookBooking(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid booking ID")
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	result, err := h.service.RebookBooking(c.Request.Context(), userID, bookingID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, result)
}

// parsePagination extracts page and limit query parameters with defaults.
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return page, limit
}
