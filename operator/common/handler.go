package common

import (
	"context"
)

// Handler Common handler interface
type Handler interface {
	Sync(ctx context.Context, namespace, name string) error
}
