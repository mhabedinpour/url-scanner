package v1handler

import (
	"context"
	"crypto/rsa"
	"fmt"
	"scanner/internal/api/specs/v1specs"
	"scanner/internal/config"
	"scanner/pkg/controller"
	"scanner/pkg/serrors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// SecHandlerOptions holds configuration for security handling, such as JWT keys.
type SecHandlerOptions struct {
	// PublicKey is the PEM-encoded RSA public key used to verify JWT signatures.
	PublicKey string
	// PrivateKey is the PEM-encoded RSA private key used for token generation (not used by server verification path).
	PrivateKey string
}

// NewSecHandlerOptions constructs SecHandlerOptions from application configuration.
func NewSecHandlerOptions(cfg *config.Config) *SecHandlerOptions {
	return &SecHandlerOptions{
		PublicKey:  cfg.JWT.PublicKey,
		PrivateKey: cfg.JWT.PrivateKey,
	}
}

// UserIDKey is the context key under which authenticated user's UUID is stored.
const UserIDKey controller.CtxKey = "userID"

// GetUserIDFromContext extracts the authenticated user's UUID from context.
// It panics if the value is missing or of unexpected type, which should not
// happen when the JWT middleware is correctly configured.
func GetUserIDFromContext(ctx context.Context) uuid.UUID {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	if !ok {
		// should happen because of the jwt middleware
		panic("userID not found in context")
	}

	return userID
}

// SecHandler verifies Bearer (JWT) tokens and enriches context with user identity.
type SecHandler struct {
	publicKey *rsa.PublicKey
}

// NewSecHandler creates a SecHandler from the provided options by parsing the RSA public key.
func NewSecHandler(options *SecHandlerOptions) (*SecHandler, error) {
	key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(options.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("could not parse public key: %w", err)
	}

	return &SecHandler{
		publicKey: key,
	}, nil
}

// Compile-time guarantee that SecHandler satisfies v1specs.SecurityHandler.
var _ v1specs.SecurityHandler = (*SecHandler)(nil)

// HandleBearerAuth validates the provided Bearer token (JWT), ensuring it is signed
// with RS256 using the configured public key, not expired, and contains a valid
// UUID subject. On success, it stores the user ID in the context.
func (s SecHandler) HandleBearerAuth(
	ctx context.Context,
	_ v1specs.OperationName,
	t v1specs.BearerAuth) (context.Context, error) {
	token, err := jwt.ParseWithClaims(t.Token, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		return s.publicKey, nil
	},
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil {
		return ctx, serrors.Wrap(serrors.ErrUnauthorized, err, "could not parse token")
	}

	if !token.Valid {
		return ctx, serrors.With(serrors.ErrUnauthorized, "invalid token")
	}

	subject, err := token.Claims.GetSubject()
	if err != nil {
		return ctx, serrors.With(serrors.ErrUnauthorized, "invalid subject")
	}

	userID, err := uuid.Parse(subject)
	if err != nil {
		return ctx, serrors.With(serrors.ErrUnauthorized, "invalid subject")
	}

	return context.WithValue(ctx, UserIDKey, userID), nil
}
