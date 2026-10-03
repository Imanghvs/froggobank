package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Imanghvs/froggobank/internal/account/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AccountService is the application capability required by the HTTP adapter.
type AccountService interface {
	CreateAccount(ctx context.Context, currencyCode string) (domain.Account, error)
	GetAccountByID(ctx context.Context, id uuid.UUID) (domain.Account, error)
}

type Handler struct {
	service AccountService
}

func New(service AccountService) *Handler {
	return &Handler{
		service: service,
	}
}

type createAccountRequest struct {
	Currency string `json:"currency"`
}

type accountResponse struct {
	ID        uuid.UUID `json:"id"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) Create(c *gin.Context) {
	var req createAccountRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	acc, err := h.service.CreateAccount(c.Request.Context(), req.Currency)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCurrency) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid currency"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create account",
		})
		return
	}

	c.JSON(http.StatusCreated, accountResponse{
		ID:        acc.ID,
		Currency:  string(acc.Currency),
		CreatedAt: acc.CreatedAt,
	})
}

func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid id",
		})
		return
	}

	acc, err := h.service.GetAccountByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to fetch account",
		})
		return
	}

	c.JSON(http.StatusOK, accountResponse{
		ID:        acc.ID,
		Currency:  string(acc.Currency),
		CreatedAt: acc.CreatedAt,
	})
}
