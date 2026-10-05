//go:build unit

package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	"github.com/viethung213/gym-companion/internal/auth/domain/event"
)

func TestSendOTPHandler(t *testing.T) {
	ctx := context.Background()
	otpRepo := newMockOTPRepo()
	hasher := &mockHasher{}
	publisher := &mockEventPublisher{}
	txMgr := &mockTxManager{}

	handler := NewSendOTPHandler(otpRepo, hasher, publisher, txMgr)

	// Test 1: Send OTP Success
	res, err := handler.Handle(ctx, SendOTPCommand{
		Identifier: "0912345678",
		Purpose:    "register",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.OTPToken == "" {
		t.Fatalf("expected success and valid otp token, got %+v", res)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 outbox event published, got %d", len(publisher.events))
	}
	otpEv, ok := publisher.events[0].(event.OTPSentEvent)
	if !ok {
		t.Fatalf("expected OTPSentEvent, got %T", publisher.events[0])
	}
	if len(otpEv.Code) != 6 {
		t.Fatalf("expected 6-digit otp code, got %s", otpEv.Code)
	}

	// Test 2: Cooldown error on immediate resend
	_, err = handler.Handle(ctx, SendOTPCommand{
		Identifier: "0912345678",
		Purpose:    "register",
	})
	if err == nil || err != derror.ErrOTPResendCooldown {
		t.Fatalf("expected ErrOTPResendCooldown, got %v", err)
	}

	// Test 3: Empty identifier validation
	_, err = handler.Handle(ctx, SendOTPCommand{
		Identifier: "",
		Purpose:    "register",
	})
	if err == nil {
		t.Fatalf("expected error on empty identifier")
	}

	// Test 4: Resend OTP after cooldown succeeds and invalidates previous active OTP
	prevOTP, err := otpRepo.FindByID(ctx, res.OTPToken)
	if err != nil {
		t.Fatalf("failed to find previous otp: %v", err)
	}
	// Reconstitute with resendAvailableAt in the past to simulate cooldown elapsed
	pastTime := time.Now().Add(-2 * time.Minute)
	expiredCooldownOTP := entity.ReconstituteOTP(
		prevOTP.ID(),
		prevOTP.Identifier(),
		prevOTP.OTPHash(),
		prevOTP.Purpose(),
		prevOTP.Attempts(),
		prevOTP.MaxAttempts(),
		pastTime.Add(5*time.Minute),
		pastTime, // resendAvailableAt in the past
		false,
		pastTime,
	)
	_ = otpRepo.Update(ctx, expiredCooldownOTP)

	res2, err := handler.Handle(ctx, SendOTPCommand{
		Identifier: "0912345678",
		Purpose:    "register",
	})
	if err != nil {
		t.Fatalf("expected resend after cooldown to succeed, got %v", err)
	}
	if !res2.Success || res2.OTPToken == res.OTPToken {
		t.Fatalf("expected new otp token, got %v", res2.OTPToken)
	}

	// Verify previous OTP was invalidated (is_used = true)
	updatedPrevOTP, _ := otpRepo.FindByID(ctx, res.OTPToken)
	if !updatedPrevOTP.IsUsed() {
		t.Fatalf("expected previous active OTP to be marked as used/invalidated, but was still unused")
	}

	// Verify new OTP is active (is_used = false)
	newOTP, _ := otpRepo.FindByID(ctx, res2.OTPToken)
	if newOTP.IsUsed() {
		t.Fatalf("expected new OTP to be active (is_used = false)")
	}
}

func TestVerifyOTPHandler(t *testing.T) {
	ctx := context.Background()
	otpRepo := newMockOTPRepo()
	resetTokenRepo := newMockPasswordResetTokenRepo()
	userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
	hasher := &mockHasher{}

	// Seed user
	user := aggregate.RegisterUser(
		"user-1",
		"Test User",
		aggregate.NewIdentity("ident-1", "email", "user@example.com", "hashed_pass", nil, time.Now(), time.Now()),
		"",
	)
	_ = userRepo.Create(ctx, user)

	handler := NewVerifyOTPHandler(otpRepo, resetTokenRepo, userRepo, hasher)

	// Seed OTP
	hashedCode, _ := hasher.Hash("123456")
	otp := entity.NewOTP("token-1", "user@example.com", hashedCode, "reset_password", 5*time.Minute, 1*time.Minute, 5)
	_ = otpRepo.Save(ctx, otp)

	// Test 1: Verify with incorrect code
	_, err := handler.Handle(ctx, VerifyOTPCommand{
		OTPToken: "token-1",
		Code:     "000000",
	})
	if err != derror.ErrOTPInvalidCode {
		t.Fatalf("expected ErrOTPInvalidCode, got %v", err)
	}

	// Test 2: Verify with correct code for reset_password (should generate reset_token)
	res, err := handler.Handle(ctx, VerifyOTPCommand{
		OTPToken: "token-1",
		Code:     "123456",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid || res.ResetToken == "" {
		t.Fatalf("expected valid status and reset_token, got %+v", res)
	}

	// Test 3: Reusing already used OTP fails
	_, err = handler.Handle(ctx, VerifyOTPCommand{
		OTPToken: "token-1",
		Code:     "123456",
	})
	if err != derror.ErrOTPAlreadyUsed {
		t.Fatalf("expected ErrOTPAlreadyUsed, got %v", err)
	}
}

func TestRegisterHandler(t *testing.T) {
	ctx := context.Background()
	userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
	hasher := &mockHasher{}
	publisher := &mockEventPublisher{}
	txMgr := &mockTxManager{}

	handler := NewRegisterHandler(userRepo, hasher, publisher, txMgr)

	// Test 1: Successful email registration
	res, err := handler.Handle(ctx, RegisterCommand{
		Identifier:  "newuser@example.com",
		Password:    "password123",
		FullName:    "New User",
		Gender:      "MALE",
		DateOfBirth: "1995-05-15",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Message == "" {
		t.Fatalf("expected success message, got empty")
	}

	// Check domain event was published with full metadata
	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(publisher.events))
	}
	ev, ok := publisher.events[0].(event.UserRegisteredEvent)
	if !ok {
		t.Fatalf("expected UserRegisteredEvent, got %T", publisher.events[0])
	}
	if ev.Gender != "MALE" || ev.DateOfBirth != "1995-05-15" || ev.Identifier != "newuser@example.com" || ev.IdentityType != "email" {
		t.Fatalf("event metadata mismatch: %+v", ev)
	}

	// Test 1b: Successful phone registration (auto-detected and normalized to +84...)
	publisher.events = nil
	resPhone, err := handler.Handle(ctx, RegisterCommand{
		Identifier:  "0912345678",
		Password:    "password123",
		FullName:    "Phone User",
		Gender:      "FEMALE",
		DateOfBirth: "2000-01-01",
	})
	if err != nil {
		t.Fatalf("unexpected error on phone register: %v", err)
	}
	if resPhone.Message == "" {
		t.Fatalf("expected success message")
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(publisher.events))
	}
	evPhone, ok := publisher.events[0].(event.UserRegisteredEvent)
	if !ok {
		t.Fatalf("expected UserRegisteredEvent, got %T", publisher.events[0])
	}
	if evPhone.IdentityType != "phone" || evPhone.Identifier != "+84912345678" {
		t.Fatalf("expected phone identity +84912345678, got %+v", evPhone)
	}

	// Test 2: Conflict error on duplicate email
	_, err = handler.Handle(ctx, RegisterCommand{
		Identifier: "newuser@example.com",
		Password:   "password123",
		FullName:   "Duplicate",
	})
	if err != derror.ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	// Test 3: Password too short
	_, err = handler.Handle(ctx, RegisterCommand{
		Identifier: "another@example.com",
		Password:   "short",
	})
	if err == nil {
		t.Fatalf("expected error for short password")
	}
}

func TestCredentialsLoginHandler(t *testing.T) {
	ctx := context.Background()
	userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
	hasher := &mockHasher{}
	keyRepo := &mockKeyRepo{
		keys: []*port.JWKRecord{
			{ID: "key-1", Status: port.KeyStatusActive},
		},
	}
	tokenService := mockTokenService{}
	sessionRepo := &mockSessionRepo{sessions: make(map[string]*port.SessionRecord)}

	// Seed user
	user := aggregate.RegisterUser(
		"user-cred-1",
		"Cred User",
		aggregate.NewIdentity("ident-cred", "email", "cred@example.com", "hashed_password123", nil, time.Now(), time.Now()),
		"",
	)
	_ = userRepo.Create(ctx, user)

	handler := NewCredentialsLoginHandler(userRepo, hasher, keyRepo, tokenService, sessionRepo)

	// Test 1: Successful login
	res, err := handler.Handle(ctx, CredentialsLoginCommand{
		Identifier: "cred@example.com",
		Password:   "password123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AccessToken != "mock-access-token" || res.RefreshToken == "" {
		t.Fatalf("invalid tokens returned: %+v", res)
	}

	// Test 2: Invalid password
	_, err = handler.Handle(ctx, CredentialsLoginCommand{
		Identifier: "cred@example.com",
		Password:   "wrong-password",
	})
	if err != derror.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}

	// Test 3: Non-existent user
	_, err = handler.Handle(ctx, CredentialsLoginCommand{
		Identifier: "nonexistent@example.com",
		Password:   "password123",
	})
	if err != derror.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}

	// Test 4: Locked user login fails with ErrUserLocked
	_ = user.Lock()
	_ = userRepo.Update(ctx, user)
	_, err = handler.Handle(ctx, CredentialsLoginCommand{
		Identifier: "cred@example.com",
		Password:   "password123",
	})
	if err != derror.ErrUserLocked {
		t.Fatalf("expected ErrUserLocked, got %v", err)
	}

	// Test 5: Suspended user login fails with ErrUserSuspended
	user.Suspend()
	_ = userRepo.Update(ctx, user)
	_, err = handler.Handle(ctx, CredentialsLoginCommand{
		Identifier: "cred@example.com",
		Password:   "password123",
	})
	if err != derror.ErrUserSuspended {
		t.Fatalf("expected ErrUserSuspended, got %v", err)
	}
}

func TestChangePasswordHandler(t *testing.T) {
	ctx := context.Background()
	userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
	hasher := &mockHasher{}

	user := aggregate.RegisterUser(
		"user-cp",
		"CP User",
		aggregate.NewIdentity("ident-cp", "email", "cp@example.com", "hashed_oldpass123", nil, time.Now(), time.Now()),
		"",
	)
	_ = userRepo.Create(ctx, user)

	handler := NewChangePasswordHandler(userRepo, hasher)

	// Test 1: Password mismatch
	_, err := handler.Handle(ctx, ChangePasswordCommand{
		UserID:          "user-cp",
		OldPassword:     "oldpass123",
		NewPassword:     "newpass123",
		ConfirmPassword: "differentpass",
	})
	if err == nil {
		t.Fatalf("expected error on password mismatch")
	}

	// Test 2: Wrong old password
	_, err = handler.Handle(ctx, ChangePasswordCommand{
		UserID:          "user-cp",
		OldPassword:     "wrongoldpass",
		NewPassword:     "newpass123",
		ConfirmPassword: "newpass123",
	})
	if err != derror.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}

	// Test 3: Success
	res, err := handler.Handle(ctx, ChangePasswordCommand{
		UserID:          "user-cp",
		OldPassword:     "oldpass123",
		NewPassword:     "newpass123",
		ConfirmPassword: "newpass123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
}

func TestResetPasswordHandler(t *testing.T) {
	ctx := context.Background()
	userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
	resetTokenRepo := newMockPasswordResetTokenRepo()
	hasher := &mockHasher{}
	txMgr := &mockTxManager{}

	user := aggregate.RegisterUser(
		"user-rp",
		"RP User",
		aggregate.NewIdentity("ident-rp", "email", "rp@example.com", "hashed_old", nil, time.Now(), time.Now()),
		"",
	)
	_ = userRepo.Create(ctx, user)

	// Seed reset token
	token := entity.NewPasswordResetToken("valid-reset-token", "user-rp", 15*time.Minute)
	_ = resetTokenRepo.Save(ctx, token)

	handler := NewResetPasswordHandler(resetTokenRepo, userRepo, hasher, txMgr)

	// Test 1: Reset password success
	res, err := handler.Handle(ctx, ResetPasswordCommand{
		ResetToken:      "valid-reset-token",
		NewPassword:     "brandnewpass123",
		ConfirmPassword: "brandnewpass123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}

	// Test 2: Token cannot be reused
	_, err = handler.Handle(ctx, ResetPasswordCommand{
		ResetToken:      "valid-reset-token",
		NewPassword:     "brandnewpass123",
		ConfirmPassword: "brandnewpass123",
	})
	if err != derror.ErrResetTokenAlreadyUsed {
		t.Fatalf("expected ErrResetTokenAlreadyUsed, got %v", err)
	}
}
