package infraerror

import "errors"

// Infrastructure Errors (Crypto & Hasher)
var (
	ErrInvalidHash         = errors.New("the encoded hash is not in the correct format")
	ErrIncompatibleVersion = errors.New("incompatible version of argon2")
	ErrMismatchedHash      = errors.New("hash does not match raw input")
)

// Infrastructure Errors (OAuth State)
var (
	ErrInvalidStateFormat    = errors.New("invalid state format")
	ErrInvalidStateSignature = errors.New("invalid state signature")
	ErrStateExpired          = errors.New("state token expired")
)
