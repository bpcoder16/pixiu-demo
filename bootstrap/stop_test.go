package bootstrap

import (
	"context"
	"errors"
	"time"
)

func stopAndWait(stop func() error, wait func(context.Context) error, cancel func()) error {
	err := stop()
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	waitErr := wait(ctx)
	if waitErr != nil {
		cancel()
	}
	return errors.Join(err, waitErr)
}
