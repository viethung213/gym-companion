package port

import "context"

// TransactionManager defines the port interface to execute application logic inside a transaction context.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// TxManager is an alias for TransactionManager for consistency and backward compatibility.
type TxManager = TransactionManager
