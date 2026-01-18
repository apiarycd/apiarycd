package auth

import "time"

type Config struct {
	SecretKey       []byte
	Issuer          string
	AccessTokenExp  time.Duration
	RefreshTokenExp time.Duration
}
