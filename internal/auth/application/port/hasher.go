package port

// Hasher defines the port for hashing and verifying sensitive credentials (e.g. passwords, OTPs).
type Hasher interface {
	Hash(raw string) (string, error)
	Compare(hashed, raw string) error
}
