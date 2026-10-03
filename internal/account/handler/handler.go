package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/Imanghvs/froggobank/internal/account"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	repository account.Repository
}

func New(repository account.Repository) *Handler {
	return &Handler{
		repository: repository,
	}
}

type createAccountRequest struct {
	Currency string `json:"currency"`
}

type accountResponse struct {
	ID        uuid.UUID        `json:"id"`
	Currency  account.Currency `json:"currency"`
	CreatedAt time.Time        `json:"created_at"`
}

func (h *Handler) Create(c *gin.Context) {
	var req createAccountRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	currency, err := account.ParseCurrency(req.Currency)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid currency",
		})
		return
	}

	acc := account.New(currency)

	if err := h.repository.Create(c.Request.Context(), acc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create account",
		})
		return
	}

	c.JSON(http.StatusCreated, accountResponse{
		ID:        acc.ID,
		Currency:  acc.Currency,
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

	acc, err := h.repository.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, account.ErrNotFound) {
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
		Currency:  acc.Currency,
		CreatedAt: acc.CreatedAt,
	})
}
