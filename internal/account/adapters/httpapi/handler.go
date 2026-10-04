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
	CreateAccount(ctx context.Context, currencyCode, accountTypeCode string, enforceNonnegativeBalance bool) (domain.Account, error)
	GetAccountByID(ctx context.Context, id uuid.UUID) (domain.Account, error)
	GetAccountBalance(ctx context.Context, id uuid.UUID) (domain.Balance, error)
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
	Currency                  string `json:"currency"`
	AccountType               string `json:"account_type"`
	EnforceNonnegativeBalance bool   `json:"enforce_nonnegative_balance"`
}

type accountResponse struct {
	ID                        uuid.UUID `json:"id"`
	Currency                  string    `json:"currency"`
	AccountType               string    `json:"account_type"`
	EnforceNonnegativeBalance bool      `json:"enforce_nonnegative_balance"`
	CreatedAt                 time.Time `json:"created_at"`
}

func (h *Handler) Create(c *gin.Context) {
	var req createAccountRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	acc, err := h.service.CreateAccount(c.Request.Context(), req.Currency, req.AccountType, req.EnforceNonnegativeBalance)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCurrency) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid currency"})
			return
		}
		if errors.Is(err, domain.ErrInvalidAccountType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account type"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create account",
		})
		return
	}

	c.JSON(http.StatusCreated, accountResponse{
		ID:                        acc.ID,
		Currency:                  string(acc.Currency),
		AccountType:               string(acc.Type),
		EnforceNonnegativeBalance: acc.EnforceNonnegativeBalance,
		CreatedAt:                 acc.CreatedAt,
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
		ID:                        acc.ID,
		Currency:                  string(acc.Currency),
		AccountType:               string(acc.Type),
		EnforceNonnegativeBalance: acc.EnforceNonnegativeBalance,
		CreatedAt:                 acc.CreatedAt,
	})
}

func (h *Handler) GetBalance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	balance, err := h.service.GetAccountBalance(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch account balance"})
		return
	}
	normalSide, _ := balance.AccountType().NormalSide()
	scale, _ := balance.Currency().Scale()
	// Decimal strings preserve integer precision in JSON clients, including
	// JavaScript clients whose numeric integers are limited to 53 bits.
	c.JSON(http.StatusOK, gin.H{
		"account_id":    balance.AccountID(),
		"currency":      balance.Currency(),
		"account_type":  balance.AccountType(),
		"normal_side":   normalSide,
		"scale":         scale,
		"debits_minor":  balance.DebitsMinor().String(),
		"credits_minor": balance.CreditsMinor().String(),
		"posted_minor":  balance.PostedMinor().String(),
	})
}
