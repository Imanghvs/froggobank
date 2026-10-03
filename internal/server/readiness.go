package server

import (
	"context"
	"time"
)

const readinessTimeout = 2 * time.Second

type DatabasePinger interface {
	Ping(context.Context) error
}
