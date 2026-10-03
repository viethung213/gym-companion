package vo_test

import (
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

func TestNewRole(t *testing.T) {
	tests := []struct {
		name    string
		give    string
		want    string
		wantErr error
	}{
		{
			name:    "Valid admin role",
			give:    "admin",
			want:    "admin",
			wantErr: nil,
		},
		{
			name:    "Valid user role",
			give:    "user",
			want:    "user",
			wantErr: nil,
		},
		{
			name:    "Valid brand role",
			give:    "brand",
			want:    "brand",
			wantErr: nil,
		},
		{
			name:    "Invalid role",
			give:    "superadmin",
			want:    "",
			wantErr: derror.ErrInvalidRole,
		},
		{
			name:    "Empty role",
			give:    "",
			want:    "",
			wantErr: derror.ErrInvalidRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vo.NewRole(tt.give)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.Value() != tt.want {
				t.Errorf("got %q, want %q", got.Value(), tt.want)
			}
		})
	}
}

func TestNewPhone(t *testing.T) {
	tests := []struct {
		name    string
		give    string
		want    string
		wantErr error
	}{
		{
			name:    "Valid domestic Vietnam 090 number",
			give:    "0901234567",
			want:    "+84901234567",
			wantErr: nil,
		},
		{
			name:    "Valid domestic Vietnam 03x with spaces and dashes",
			give:    "038-123 4567",
			want:    "+84381234567",
			wantErr: nil,
		},
		{
			name:    "Valid domestic Vietnam starting with 84 without plus",
			give:    "84912345678",
			want:    "+84912345678",
			wantErr: nil,
		},
		{
			name:    "Valid international E.164 already",
			give:    "+14155552671",
			want:    "+14155552671",
			wantErr: nil,
		},
		{
			name:    "Invalid empty phone",
			give:    "",
			want:    "",
			wantErr: derror.ErrInvalidPhone,
		},
		{
			name:    "Invalid letters in phone",
			give:    "09012abcde",
			want:    "",
			wantErr: derror.ErrInvalidPhone,
		},
		{
			name:    "Invalid too short phone",
			give:    "1234",
			want:    "",
			wantErr: derror.ErrInvalidPhone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vo.NewPhone(tt.give)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				if got.Value() != tt.want {
					t.Errorf("got %q, want %q", got.Value(), tt.want)
				}
				if got.IsZero() {
					t.Errorf("got IsZero true, want false")
				}
			}
		})
	}

	var zeroPhone vo.Phone
	if !zeroPhone.IsZero() {
		t.Errorf("got IsZero false for zero value, want true")
	}
}

func TestNewEmail(t *testing.T) {
	tests := []struct {
		name    string
		give    string
		want    string
		wantErr error
	}{
		{
			name:    "Valid email trimmed and lowered",
			give:    " Test@Example.Com ",
			want:    "test@example.com",
			wantErr: nil,
		},
		{
			name:    "Invalid email format",
			give:    "plainaddress",
			want:    "",
			wantErr: derror.ErrInvalidEmail,
		},
		{
			name:    "Empty email",
			give:    "",
			want:    "",
			wantErr: derror.ErrInvalidEmail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vo.NewEmail(tt.give)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.Value() != tt.want {
				t.Errorf("got %q, want %q", got.Value(), tt.want)
			}
		})
	}
}

func TestRawPassword(t *testing.T) {
	t.Run("Valid password", func(t *testing.T) {
		p, err := vo.NewRawPassword("password123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Value() != "password123" {
			t.Errorf("got %s, want %s", p.Value(), "password123")
		}
		if p.IsZero() {
			t.Errorf("expected IsZero false")
		}
	})

	t.Run("Too short password", func(t *testing.T) {
		_, err := vo.NewRawPassword("short")
		if !errors.Is(err, derror.ErrPasswordTooShort) {
			t.Fatalf("got %v, want %v", err, derror.ErrPasswordTooShort)
		}
	})

	t.Run("Confirmed matching password", func(t *testing.T) {
		p, err := vo.NewConfirmedRawPassword("password123", "password123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Value() != "password123" {
			t.Errorf("got %s, want %s", p.Value(), "password123")
		}
	})

	t.Run("Confirmed mismatched password", func(t *testing.T) {
		_, err := vo.NewConfirmedRawPassword("password123", "different123")
		if !errors.Is(err, derror.ErrPasswordMismatch) {
			t.Fatalf("got %v, want %v", err, derror.ErrPasswordMismatch)
		}
	})

	t.Run("Confirmed too short password", func(t *testing.T) {
		_, err := vo.NewConfirmedRawPassword("short", "short")
		if !errors.Is(err, derror.ErrPasswordTooShort) {
			t.Fatalf("got %v, want %v", err, derror.ErrPasswordTooShort)
		}
	})

	var zeroRaw vo.RawPassword
	if !zeroRaw.IsZero() {
		t.Errorf("expected IsZero true for uninitialized RawPassword")
	}
}

func TestHashedPassword(t *testing.T) {
	t.Run("Valid hash", func(t *testing.T) {
		h, err := vo.NewHashedPassword("$argon2id$v=19$m=65536,t=3,p=2$mockhash")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h.Value() != "$argon2id$v=19$m=65536,t=3,p=2$mockhash" {
			t.Errorf("got %s, want %s", h.Value(), "$argon2id$v=19$m=65536,t=3,p=2$mockhash")
		}
		if h.IsZero() {
			t.Errorf("expected IsZero false")
		}
	})

	t.Run("Empty or whitespace hash", func(t *testing.T) {
		_, err := vo.NewHashedPassword("   ")
		if !errors.Is(err, derror.ErrEmptyHashedPassword) {
			t.Fatalf("got %v, want %v", err, derror.ErrEmptyHashedPassword)
		}
	})

	var zeroHashed vo.HashedPassword
	if !zeroHashed.IsZero() {
		t.Errorf("expected IsZero true for uninitialized HashedPassword")
	}
}
