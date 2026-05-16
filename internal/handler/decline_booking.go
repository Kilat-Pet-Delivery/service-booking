package handler

import (
	"net/http"

	"github.com/Kilat-Pet-Delivery/lib-common/domain"
	"github.com/Kilat-Pet-Delivery/lib-common/middleware"
	"github.com/Kilat-Pet-Delivery/lib-common/response"
	"github.com/Kilat-Pet-Delivery/lib-proto/dto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DeclineBooking handles POST /api/v1/bookings/:id/decline.
// Only the runner assigned to the booking may decline it.
func (h *BookingHandler) DeclineBooking(c *gin.Context) {
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

	var req dto.DeclineBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		response.Error(c, domain.NewValidationError(err.Error()))
		return
	}

	result, err := h.service.DeclineBooking(c.Request.Context(), bookingID, runnerID, req.Reason)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}
