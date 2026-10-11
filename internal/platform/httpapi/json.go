package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin/binding"
)

const maxJSONBodyBytes = 1 << 20

// StrictJSON binds a single JSON value, rejecting unknown fields and bodies
// larger than 1 MiB. It also validates Gin binding tags.
type StrictJSON struct{}

func (StrictJSON) Name() string { return "json" }

func (StrictJSON) Bind(req *http.Request, dst any) error {
	if req == nil || req.Body == nil {
		return errors.New("missing request body")
	}

	decoder := json.NewDecoder(http.MaxBytesReader(nil, req.Body, maxJSONBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var extra any
	switch err := decoder.Decode(&extra); err {
	case io.EOF:
	case nil:
		return errors.New("request body must contain only one JSON value")
	default:
		return err
	}

	if binding.Validator != nil {
		return binding.Validator.ValidateStruct(dst)
	}
	return nil
}
