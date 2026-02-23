package processors

import "context"

// Worker defines the contract for background processes.
type Worker interface {
	Start(ctx context.Context) error
}
