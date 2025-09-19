package domain

import "github.com/google/uuid"

// UserID uniquely identifies a user within the system.
// It is a thin wrapper around uuid.UUID to provide type safety at the domain layer.
type UserID uuid.UUID
