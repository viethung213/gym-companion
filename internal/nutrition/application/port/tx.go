package port

import "context"

// TransactionManager defines port for running business operations in a DB transaction context.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// TxManager is an alias for TransactionManager for compatibility.
type TxManager = TransactionManager
