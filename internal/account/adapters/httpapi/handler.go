package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Imanghvs/froggobank/internal/account/application"
	"github.com/Imanghvs/froggobank/internal/account/domain"
	httpresponse "github.com/Imanghvs/froggobank/internal/platform/httpapi"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

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
	ID                        uuid.UUID `json:"id"`
	Currency                  string    `json:"currency"`
	AccountType               string    `json:"account_type"`
	EnforceNonnegativeBalance bool      `json:"enforce_nonnegative_balance"`
	CreatedAt                 time.Time `json:"created_at"`
}

type paginatedAccountsResponse struct {
	Accounts []accountResponse `json:"accounts"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

func (h *Handler) Create(c *gin.Context) {
	caller, ok := httpresponse.RequireUser(c)
	if !ok {
		return
	}
	var req createAccountRequest

	if err := c.ShouldBindWith(&req, httpresponse.StrictJSON{}); err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	acc, err := h.service.CreateAccount(c.Request.Context(), caller.ID, req.Currency)
	if err != nil {
		if errors.Is(err, user.ErrUnauthenticated) {
			httpresponse.Unauthorized(c)
			return
		}
		if errors.Is(err, domain.ErrInvalidCurrency) {
			httpresponse.Error(c, http.StatusBadRequest, "invalid currency")
			return
		}
		httpresponse.Error(c, http.StatusInternalServerError, "failed to create account")
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
	caller, ok := httpresponse.RequireUser(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	acc, err := h.service.GetAccountByID(c.Request.Context(), caller.ID, id)
	if err != nil {
		if errors.Is(err, user.ErrUnauthenticated) {
			httpresponse.Unauthorized(c)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			httpresponse.Error(c, http.StatusNotFound, "account not found")
			return
		}
		httpresponse.Error(c, http.StatusInternalServerError, "failed to fetch account")
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

func (h *Handler) GetUserAccounts(c *gin.Context) {
	caller, ok := httpresponse.RequireUser(c)
	if !ok {
		return
	}
	var query getUserAccountsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "limit must be 1-100 and offset must be nonnegative integers")
		return
	}

	var currency *domain.Currency
	if query.Currency != nil {
		parsed, err := domain.ParseCurrency(*query.Currency)
		if err != nil {
			httpresponse.Error(c, http.StatusBadRequest, "invalid currency")
			return
		}
		currency = &parsed
	}

	res, err := h.service.GetUserAccounts(
		c.Request.Context(),
		caller.ID,
		application.AccountFilter{
			Limit:    query.Limit,
			Offset:   query.Offset,
			Currency: currency,
		},
	)
	if err != nil {
		if errors.Is(err, user.ErrUnauthenticated) {
			httpresponse.Unauthorized(c)
			return
		}
		httpresponse.Error(c, http.StatusInternalServerError, "failed to fetch account")
		return
	}

	response := paginatedAccountsResponse{
		Accounts: make([]accountResponse, 0, len(res.Accounts)),
		Limit:    res.Limit,
		Offset:   res.Offset,
	}
	for _, acc := range res.Accounts {
		response.Accounts = append(response.Accounts, accountResponse{
			ID:                        acc.ID,
			Currency:                  string(acc.Currency),
			AccountType:               string(acc.Type),
			EnforceNonnegativeBalance: acc.EnforceNonnegativeBalance,
			CreatedAt:                 acc.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) GetBalance(c *gin.Context) {
	caller, ok := httpresponse.RequireUser(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	balance, err := h.service.GetAccountBalance(c.Request.Context(), caller.ID, id)
	if err != nil {
		if errors.Is(err, user.ErrUnauthenticated) {
			httpresponse.Unauthorized(c)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			httpresponse.Error(c, http.StatusNotFound, "account not found")
			return
		}
		httpresponse.Error(c, http.StatusInternalServerError, "failed to fetch account balance")
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
