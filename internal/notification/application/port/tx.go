package port

import "context"

// TransactionManager defines the port interface to execute application logic inside a transaction context.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// TxManager defines the transaction manager port including legacy ExecTx for backward compatibility.
type TxManager interface {
	TransactionManager
	ExecTx(ctx context.Context, fn func(ctx context.Context) error) error
}
