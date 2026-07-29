package port

import "context"

// TransactionManager định nghĩa port giúp thực thi logic ứng dụng trong 1 transaction context.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// TxManager là alias cho TransactionManager để tương thích ngược.
type TxManager = TransactionManager
