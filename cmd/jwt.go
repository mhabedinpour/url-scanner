package main

import (
	"context"
	"fmt"
	"scanner/internal/config"
	"scanner/pkg/logger"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// JWTCommand constructs the 'jwt' subcommand that generates a signed RS256 JWT
// for a given subject (user ID) and TTL using the configured private key.
func JWTCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jwt",
		Short: "Generates JWT token for given user ID",
		Run: func(cmd *cobra.Command, args []string) {
			subject, _ := cmd.Flags().GetString("subject")
			TTL, _ := cmd.Flags().GetDuration("ttl")

			key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.JWT.PrivateKey))
			if err != nil {
				logger.Fatal(context.Background(), "could not parse RSA private key", zap.Error(err))
			}

			claims := jwt.RegisteredClaims{
				Subject:   subject,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(TTL)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
			}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			signed, err := token.SignedString(key)
			if err != nil {
				logger.Fatal(context.Background(), "could not sign JWT", zap.Error(err))
			}

			fmt.Println(signed) //nolint: forbidigo
		},
	}

	cmd.Flags().String("subject", "", "JWT subject (e.g., user ID)")
	cmd.Flags().Duration("ttl", 24*time.Hour, "Token TTL (e.g., 30s, 15m, 1h)")
	_ = cmd.MarkFlagRequired("subject")

	return cmd
}
