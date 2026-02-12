package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Kilat-Pet-Delivery/lib-common/auth"
	"github.com/Kilat-Pet-Delivery/lib-common/middleware"
	"github.com/Kilat-Pet-Delivery/lib-common/response"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/application"
)

// AdminBookingHandler handles admin HTTP requests for booking management.
type AdminBookingHandler struct {
	service *application.BookingService
}

// NewAdminBookingHandler creates a new AdminBookingHandler.
func NewAdminBookingHandler(service *application.BookingService) *AdminBookingHandler {
	return &AdminBookingHandler{service: service}
}

// RegisterRoutes registers admin booking routes.
func (h *AdminBookingHandler) RegisterRoutes(r *gin.RouterGroup, jwtManager *auth.JWTManager) {
	authMW := middleware.AuthMiddleware(jwtManager)
	adminRole := middleware.RequireRole(auth.RoleAdmin)

	admin := r.Group("/api/v1/admin")
	admin.Use(authMW, adminRole)
	{
		admin.GET("/bookings", h.ListBookings)
		admin.GET("/stats/bookings", h.BookingStats)
	}
}

// ListBookings handles GET /api/v1/admin/bookings.
func (h *AdminBookingHandler) ListBookings(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	bookings, total, err := h.service.ListAllBookings(c.Request.Context(), page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Paginated(c, bookings, total, page, limit)
}

// BookingStats handles GET /api/v1/admin/stats/bookings.
func (h *AdminBookingHandler) BookingStats(c *gin.Context) {
	stats, err := h.service.GetBookingStats(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, stats)
}
