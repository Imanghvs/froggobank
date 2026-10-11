package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	httpresponse "github.com/Imanghvs/froggobank/internal/platform/httpapi"
	"github.com/Imanghvs/froggobank/internal/transfer/domain"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

type Handler struct {
	service TransferService
}

type transferRequest struct {
	Source      uuid.UUID `json:"source"`
	Destination uuid.UUID `json:"destination"`
	AmountMinor string    `json:"amount_minor"`
}

type transferResponse struct {
	ID          uuid.UUID `json:"id"`
	Source      uuid.UUID `json:"source"`
	Destination uuid.UUID `json:"destination"`
	Currency    string    `json:"currency"`
	AmountMinor string    `json:"amount_minor"`
	CreatedAt   time.Time `json:"created_at"`
}

func New(service TransferService) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Transfer(c *gin.Context) {
	caller, ok := httpresponse.RequireUser(c)
	if !ok {
		return
	}
	var req transferRequest
	if err := c.ShouldBindWith(&req, httpresponse.StrictJSON{}); err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	tr, err := h.service.Transfer(
		c.Request.Context(),
		caller.ID,
		req.Source,
		req.Destination,
		req.AmountMinor,
	)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrUnauthenticated):
			httpresponse.Unauthorized(c)
		case errors.Is(err, domain.ErrInvalidCurrency):
			httpresponse.Error(c, http.StatusBadRequest, "invalid currency")
		case errors.Is(err, domain.ErrInvalidAmount):
			httpresponse.Error(c, http.StatusBadRequest, "invalid amount")
		case errors.Is(err, domain.ErrInvalidAccountID):
			httpresponse.Error(c, http.StatusBadRequest, "invalid source or destination account ID")
		case errors.Is(err, domain.ErrSameAccount):
			httpresponse.Error(c, http.StatusBadRequest, "source and destination accounts must be different")
		case errors.Is(err, domain.ErrSourceNotFound):
			httpresponse.Error(c, http.StatusNotFound, "source account not found")
		case errors.Is(err, domain.ErrDestinationNotFound):
			httpresponse.Error(c, http.StatusNotFound, "destination account not found")
		case errors.Is(err, domain.ErrCurrencyMismatch):
			httpresponse.Error(c, http.StatusBadRequest, "account currencies must match the transfer currency")
		default:
			httpresponse.Error(c, http.StatusInternalServerError, "failed to create transfer")
		}
		return
	}
	c.JSON(http.StatusCreated, transferResponse{
		ID:          tr.ID,
		Currency:    string(tr.Currency),
		Source:      tr.Source,
		Destination: tr.Destination,
		AmountMinor: strconv.FormatInt(tr.AmountMinor.MinorUnits(), 10),
		CreatedAt:   tr.CreatedAt,
	})
}
