package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// RegisterCommand contains user registration data.
type RegisterCommand struct {
	Identifier  string // Email address or Domestic/E.164 Phone
	Password    string
	FullName    string
	Gender      string
	DateOfBirth string
}

// RegisterResult contains result of registration.
type RegisterResult struct {
	Message string
}

// RegisterHandler processes user sign-up requests.
type RegisterHandler struct {
	userRepo     repository.UserRepository
	hasher       port.Hasher
	outboxWriter port.OutboxWriter
	txManager    port.TxManager
}

// NewRegisterHandler creates a new RegisterHandler.
func NewRegisterHandler(
	userRepo repository.UserRepository,
	hasher port.Hasher,
	outboxWriter port.OutboxWriter,
	txManager port.TxManager,
) *RegisterHandler {
	return &RegisterHandler{
		userRepo:     userRepo,
		hasher:       hasher,
		outboxWriter: outboxWriter,
		txManager:    txManager,
	}
}

// Handle creates a new user, hashes their credentials, and publishes a UserRegistered event.
func (h *RegisterHandler) Handle(ctx context.Context, cmd RegisterCommand) (*RegisterResult, error) {
	rawIdentifier := strings.TrimSpace(cmd.Identifier)
	if rawIdentifier == "" {
		return nil, fmt.Errorf("identifier cannot be empty")
	}

	passwordVO, err := vo.NewRawPassword(cmd.Password)
	if err != nil {
		return nil, err
	}

	// Tự động nhận diện Email hoặc Số điện thoại
	var identityType, identifier string
	if strings.Contains(rawIdentifier, "@") {
		emailVO, err := vo.NewEmail(rawIdentifier)
		if err != nil {
			return nil, fmt.Errorf("invalid email format: %w", err)
		}
		identityType = aggregate.IdentityTypeEmail
		identifier = emailVO.Value()
	} else {
		phoneVO, err := vo.NewPhone(rawIdentifier)
		if err != nil {
			return nil, fmt.Errorf("invalid identifier: must be a valid email or phone number (%w)", err)
		}
		identityType = aggregate.IdentityTypePhone
		identifier = phoneVO.Value()
	}

	// 1. Kiểm tra tài khoản đã tồn tại chưa trước khi vào Transaction (Fail-fast, tránh tốn CPU hash password)
	existingUser, err := h.userRepo.FindByIdentity(ctx, identityType, identifier)
	if err != nil && !errors.Is(err, derror.ErrNotFound) && !errors.Is(err, derror.ErrUserNotFound) {
		return nil, fmt.Errorf("check existing identity: %w", err)
	}
	if existingUser != nil {
		return nil, derror.ErrConflict
	}

	// 2. Băm mật khẩu bằng Argon2id (thực hiện trước khi mở transaction để tránh giữ DB connection lâu)
	hashedPassword, err := h.hasher.Hash(passwordVO.Value())
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	hashedVO, err := vo.NewHashedPassword(hashedPassword)
	if err != nil {
		return nil, fmt.Errorf("invalid hashed password: %w", err)
	}

	userID := uuid.New().String()
	identityID := uuid.New().String()
	now := time.Now()

	identity := aggregate.NewIdentity(
		identityID,
		identityType,
		identifier,
		hashedVO.Value(),
		nil,
		now,
		now,
	)

	user := aggregate.RegisterNewUser(
		userID,
		cmd.FullName,
		cmd.Gender,
		cmd.DateOfBirth,
		identity,
		"",
	)

	// 3. Transaction chỉ thực hiện bọc các thao tác thêm mới (Create User & Write Outbox)
	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.userRepo.Create(txCtx, user); err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		for _, ev := range user.DomainEvents() {
			if err := h.outboxWriter.Write(txCtx, ev); err != nil {
				return fmt.Errorf("write outbox event: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return &RegisterResult{
		Message: "Đăng ký tài khoản thành công",
	}, nil
}
