package dynapi

import "context"

type recordBackend interface {
	read(ctx context.Context, name string, rrtype uint16) (recordSet, error)
	replace(ctx context.Context, name string, rrtype uint16, records recordSet) error
	delete(ctx context.Context, name string, rrtype uint16) error
}
