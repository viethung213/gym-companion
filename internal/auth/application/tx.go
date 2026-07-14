package application

import "context"

// TransactionManager defines the port interface to execute application logic inside a transaction context.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
