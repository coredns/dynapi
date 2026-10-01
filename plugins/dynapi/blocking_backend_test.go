package dynapi

import (
	"context"
	"fmt"
)

type blockingBackend struct {
	started chan struct{}
	release chan struct{}
}

func (blockingBackend *blockingBackend) read(
	ctx context.Context,
	_ string,
	_ uint16,
) (recordSet, error) {
	blockingBackend.started <- struct{}{}

	select {
	case <-blockingBackend.release:
		return recordSet{TTL: 60, Addresses: []string{"192.0.2.1"}}, nil
	case <-ctx.Done():
		return recordSet{}, fmt.Errorf("wait for test backend: %w", ctx.Err())
	}
}

func (*blockingBackend) replace(context.Context, string, uint16, recordSet) error { return nil }

func (*blockingBackend) delete(context.Context, string, uint16) error { return nil }
