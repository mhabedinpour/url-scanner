package v1handler_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"scanner/pkg/domain"
	"testing"
	"time"

	"scanner/internal/api/handler/v1handler"
	"scanner/internal/api/specs/v1specs"
	"scanner/pkg/serrors"

	"github.com/stretchr/testify/require"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// helper to generate an RSA key pair and return the private key and PEM-encoded public key.
func genRSAKeys(tb testing.TB) (*rsa.PrivateKey, string) {
	tb.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(tb, err, "failed to generate RSA key")
	pubASN1, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(tb, err, "failed to marshal public key")
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubASN1})

	return priv, string(pubPEM)
}

func newSecHandlerForTest(t *testing.T, pubPEM string) *v1handler.SecHandler {
	t.Helper()
	sh, err := v1handler.NewSecHandler(&v1handler.SecHandlerOptions{PublicKey: pubPEM})
	require.NoError(t, err, "NewSecHandler failed")

	return sh
}

func signJWTRS256(tb testing.TB, priv *rsa.PrivateKey, sub string, issuedAt time.Time, exp time.Time) string {
	tb.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(exp),
		NotBefore: jwt.NewNumericDate(issuedAt),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(priv)
	require.NoError(tb, err, "failed to sign token")

	return signed
}

func TestHandleBearerAuth_ValidToken(t *testing.T) {
	priv, pubPEM := genRSAKeys(t)
	sh := newSecHandlerForTest(t, pubPEM)

	uid := uuid.New()
	now := time.Now()
	tkn := signJWTRS256(t, priv, uid.String(), now, now.Add(1*time.Hour))

	ctx, err := sh.HandleBearerAuth(context.Background(), "", v1specs.BearerAuth{Token: tkn})
	require.NoError(t, err)

	// verify user id stored in context
	v := ctx.Value(v1handler.UserIDKey)
	require.NotNil(t, v, "expected userID in context")
	got, ok := v.(domain.UserID)
	require.True(t, ok, "userID in context has wrong type: %T", v)
	require.Equal(t, domain.UserID(uid), got)
}

func TestHandleBearerAuth_InvalidSignature(t *testing.T) {
	// handler uses pub from key A, but token signed with key B
	_, pubPEM := genRSAKeys(t)
	sh := newSecHandlerForTest(t, pubPEM)

	privOther, _ := genRSAKeys(t)
	now := time.Now()
	tkn := signJWTRS256(t, privOther, uuid.NewString(), now, now.Add(time.Hour))

	_, err := sh.HandleBearerAuth(context.Background(), "", v1specs.BearerAuth{Token: tkn})
	require.Error(t, err)
	require.ErrorIs(t, err, serrors.ErrUnauthorized)
}

func TestHandleBearerAuth_ExpiredToken(t *testing.T) {
	priv, pubPEM := genRSAKeys(t)
	sh := newSecHandlerForTest(t, pubPEM)

	now := time.Now()
	tkn := signJWTRS256(t, priv, uuid.NewString(), now.Add(-2*time.Hour), now.Add(-1*time.Hour))

	_, err := sh.HandleBearerAuth(context.Background(), "", v1specs.BearerAuth{Token: tkn})
	require.Error(t, err)
	require.ErrorIs(t, err, serrors.ErrUnauthorized)
}

func TestHandleBearerAuth_InvalidSubject(t *testing.T) {
	priv, pubPEM := genRSAKeys(t)
	sh := newSecHandlerForTest(t, pubPEM)

	now := time.Now()
	// non-UUID subject
	tkn := signJWTRS256(t, priv, "not-a-uuid", now, now.Add(time.Hour))

	_, err := sh.HandleBearerAuth(context.Background(), "", v1specs.BearerAuth{Token: tkn})
	require.Error(t, err)
	require.ErrorIs(t, err, serrors.ErrUnauthorized)
}

func TestHandleBearerAuth_WrongAlgorithm(t *testing.T) {
	// create handler with RSA public key, but sign token with HS256
	_, pubPEM := genRSAKeys(t)
	sh := newSecHandlerForTest(t, pubPEM)

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   uuid.NewString(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		NotBefore: jwt.NewNumericDate(now),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("secret"))
	require.NoError(t, err, "failed to sign HS256 token")

	_, err = sh.HandleBearerAuth(context.Background(), "", v1specs.BearerAuth{Token: signed})
	require.Error(t, err)
	require.ErrorIs(t, err, serrors.ErrUnauthorized)
}
