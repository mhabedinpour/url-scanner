// Package storage defines the core storage interfaces that the application relies on.
// It abstracts persistence operations and transaction management so that different
// backends (e.g. PostgreSQL) can provide concrete implementations.
//
//go:generate mockgen -package mockstorage -source=interface.go -destination=mock/mockstorage.go *
package storage

import "context"

// AllStorage is a composite interface that includes all domain-specific storage
// capabilities required by the application. Implementations typically embed
// other narrower interfaces such as ScanStorage.
type AllStorage interface {
	ScanStorage
	JobStorage
}

// TxStorage describes a storage handle that operates within a database
// transaction. It exposes the same domain-specific capabilities as AllStorage,
// and additionally allows committing or rolling back the ongoing transaction.
// Implementations should become unusable after Commit or Rollback is called.
type TxStorage interface {
	AllStorage

	// Commit finalizes the transaction, persisting all changes.
	Commit() error
	// Rollback aborts the transaction, discarding all uncommitted changes.
	Rollback() error
}

// Storage describes a non-transactional storage handle with the ability to
// start transactions. It exposes domain-specific capabilities and lifecycle
// management such as Close.
type Storage interface {
	AllStorage

	// Close releases any resources held by the storage implementation (e.g. the
	// underlying connection pool). After Close, the instance should not be used.
	Close() error

	// Begin starts a new transaction and returns a TxStorage that can be used to
	// perform further operations within that transaction.
	Begin(ctx context.Context) (TxStorage, error)
	// WithTx is a helper that begins a transaction, invokes the provided callback
	// with a TxStorage, and then commits on success or rolls back if the callback
	// returns an error.
	WithTx(ctx context.Context, cb func(storage AllStorage) error) error
}
