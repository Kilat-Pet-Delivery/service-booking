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

// PetHandler handles HTTP requests for pet profile operations.
type PetHandler struct {
	service *application.PetService
}

// NewPetHandler creates a new PetHandler.
func NewPetHandler(service *application.PetService) *PetHandler {
	return &PetHandler{service: service}
}

// RegisterRoutes registers all pet profile routes.
func (h *PetHandler) RegisterRoutes(r *gin.RouterGroup, jwtManager *auth.JWTManager) {
	authMW := middleware.AuthMiddleware(jwtManager)
	ownerRole := middleware.RequireRole(auth.RoleOwner)

	pets := r.Group("/api/v1/pets")
	pets.Use(authMW, ownerRole)
	{
		pets.POST("", h.CreatePet)
		pets.GET("", h.GetMyPets)
		pets.GET("/:id", h.GetPet)
		pets.PUT("/:id", h.UpdatePet)
		pets.DELETE("/:id", h.DeletePet)
	}
}

// CreatePet creates a new pet profile.
func (h *PetHandler) CreatePet(c *gin.Context) {
	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req application.CreatePetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.CreatePet(c.Request.Context(), ownerID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": result})
}

// GetMyPets returns all pet profiles for the current owner.
func (h *PetHandler) GetMyPets(c *gin.Context) {
	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	result, err := h.service.GetMyPets(c.Request.Context(), ownerID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// GetPet returns a single pet profile by ID.
func (h *PetHandler) GetPet(c *gin.Context) {
	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	petID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid pet ID")
		return
	}

	result, err := h.service.GetPet(c.Request.Context(), ownerID, petID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// UpdatePet updates a pet profile.
func (h *PetHandler) UpdatePet(c *gin.Context) {
	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	petID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid pet ID")
		return
	}

	var req application.UpdatePetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.UpdatePet(c.Request.Context(), ownerID, petID, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, result)
}

// DeletePet archives a pet profile.
func (h *PetHandler) DeletePet(c *gin.Context) {
	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	petID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid pet ID")
		return
	}

	if err := h.service.DeletePet(c.Request.Context(), ownerID, petID); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"message": "pet profile archived"})
}
