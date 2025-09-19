package storage

import "errors"

// Common errors returned by storage implementations.
var (
	// ErrAlreadyInTx is returned when an operation requiring a non-transactional
	// context is attempted while already inside a transaction.
	ErrAlreadyInTx = errors.New("already in tx")
	// ErrNotInTx is returned when a transaction-specific operation is attempted
	// while not currently inside a transaction.
	ErrNotInTx = errors.New("not in tx")
)
