package entity

import (
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

// OTP represents an OTP verification entity.
type OTP struct {
	id                string
	identifier        string
	otpHash           string
	purpose           string
	attempts          int
	maxAttempts       int
	expiresAt         time.Time
	resendAvailableAt time.Time
	isUsed            bool
	createdAt         time.Time
}

// NewOTP creates a new OTP domain entity.
func NewOTP(
	id string,
	identifier string,
	otpHash string,
	purpose string,
	ttl time.Duration,
	cooldown time.Duration,
	maxAttempts int,
) *OTP {
	now := time.Now()
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	return &OTP{
		id:                id,
		identifier:        identifier,
		otpHash:           otpHash,
		purpose:           purpose,
		attempts:          0,
		maxAttempts:       maxAttempts,
		expiresAt:         now.Add(ttl),
		resendAvailableAt: now.Add(cooldown),
		isUsed:            false,
		createdAt:         now,
	}
}

// ReconstituteOTP restores an OTP entity from persistence.
func ReconstituteOTP(
	id string,
	identifier string,
	otpHash string,
	purpose string,
	attempts int,
	maxAttempts int,
	expiresAt time.Time,
	resendAvailableAt time.Time,
	isUsed bool,
	createdAt time.Time,
) *OTP {
	return &OTP{
		id:                id,
		identifier:        identifier,
		otpHash:           otpHash,
		purpose:           purpose,
		attempts:          attempts,
		maxAttempts:       maxAttempts,
		expiresAt:         expiresAt,
		resendAvailableAt: resendAvailableAt,
		isUsed:            isUsed,
		createdAt:         createdAt,
	}
}

// Verify validates the plain code against the stored hash.
// If valid, marks the OTP as used. If invalid, increments attempts.
func (o *OTP) Verify(plainCode string, compareHash func(plain, hash string) bool, now time.Time) error {
	if o.isUsed {
		return derror.ErrOTPAlreadyUsed
	}

	if now.After(o.expiresAt) {
		return derror.ErrOTPExpired
	}

	if o.attempts >= o.maxAttempts {
		return derror.ErrOTPMaxAttemptsExceeded
	}

	if !compareHash(plainCode, o.otpHash) {
		o.attempts++
		return derror.ErrOTPInvalidCode
	}

	o.isUsed = true
	return nil
}

// Invalidate marks the OTP as superseded or cancelled only if cooldown period has elapsed.
func (o *OTP) Invalidate(now time.Time) error {
	if !o.CanResend(now) {
		return derror.ErrOTPResendCooldown
	}
	o.isUsed = true
	return nil
}

// CanResend checks if the cooldown period has elapsed.
func (o *OTP) CanResend(now time.Time) bool {
	return now.After(o.resendAvailableAt) || now.Equal(o.resendAvailableAt)
}

// Getters
func (o *OTP) ID() string                   { return o.id }
func (o *OTP) Identifier() string           { return o.identifier }
func (o *OTP) OTPHash() string              { return o.otpHash }
func (o *OTP) Purpose() string              { return o.purpose }
func (o *OTP) Attempts() int                { return o.attempts }
func (o *OTP) MaxAttempts() int             { return o.maxAttempts }
func (o *OTP) ExpiresAt() time.Time         { return o.expiresAt }
func (o *OTP) ResendAvailableAt() time.Time { return o.resendAvailableAt }
func (o *OTP) IsUsed() bool                 { return o.isUsed }
func (o *OTP) CreatedAt() time.Time         { return o.createdAt }
