package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/infraerror"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHash         = infraerror.ErrInvalidHash
	ErrIncompatibleVersion = infraerror.ErrIncompatibleVersion
	ErrMismatchedHash      = infraerror.ErrMismatchedHash
)

// Argon2Params defines tunable parameters for Argon2id hashing according to RFC 9106.
type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2Params returns production-grade secure parameters.
func DefaultArgon2Params() Argon2Params {
	return Argon2Params{
		Memory:      64 * 1024, // 64 MB
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// TestArgon2Params returns lightweight parameters for rapid testing without compromising logic.
func TestArgon2Params() Argon2Params {
	return Argon2Params{
		Memory:      8 * 1024, // 8 MB
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// Argon2Hasher implements the port.Hasher interface using the Argon2id key derivation function.
type Argon2Hasher struct {
	params Argon2Params
}

var _ port.Hasher = (*Argon2Hasher)(nil)

// NewArgon2Hasher creates an Argon2Hasher with default production parameters.
func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{params: DefaultArgon2Params()}
}

// NewCustomArgon2Hasher creates an Argon2Hasher with custom parameters.
func NewCustomArgon2Hasher(params Argon2Params) *Argon2Hasher {
	return &Argon2Hasher{params: params}
}

// Hash generates an Argon2id PHC-formatted string from raw text.
func (h *Argon2Hasher) Hash(raw string) (string, error) {
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(raw),
		salt,
		h.params.Iterations,
		h.params.Memory,
		h.params.Parallelism,
		h.params.KeyLength,
	)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.params.Memory,
		h.params.Iterations,
		h.params.Parallelism,
		b64Salt,
		b64Hash,
	)

	return encoded, nil
}

// Compare checks whether raw text matches an Argon2id PHC-formatted string in constant time.
func (h *Argon2Hasher) Compare(hashed, raw string) error {
	parts := strings.Split(hashed, "$")
	if len(parts) != 6 {
		return ErrInvalidHash
	}

	if parts[1] != "argon2id" {
		return ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return fmt.Errorf("parse argon2 version: %w", err)
	}
	if version != argon2.Version {
		return ErrIncompatibleVersion
	}

	var memory, iterations uint32
	var parallelism uint8
	paramPairs := strings.Split(parts[3], ",")
	for _, pair := range paramPairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			return ErrInvalidHash
		}
		val, err := strconv.ParseUint(kv[1], 10, 32)
		if err != nil {
			return fmt.Errorf("parse parameter %s: %w", kv[0], err)
		}
		switch kv[0] {
		case "m":
			memory = uint32(val)
		case "t":
			iterations = uint32(val)
		case "p":
			parallelism = uint8(val)
		}
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("decode salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("decode hash: %w", err)
	}

	comparisonHash := argon2.IDKey(
		[]byte(raw),
		salt,
		iterations,
		memory,
		parallelism,
		uint32(len(expectedHash)),
	)

	if subtle.ConstantTimeCompare(comparisonHash, expectedHash) != 1 {
		return ErrMismatchedHash
	}

	return nil
}
