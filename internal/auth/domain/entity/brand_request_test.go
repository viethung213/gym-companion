package entity

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

func TestNewBrandRequest(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		id           string
		userID       string
		brandName    string
		description  string
		contactPhone string
		address      string
		wantErr      error
	}{
		{
			name:         "valid brand request",
			id:           "req-1",
			userID:       "user-1",
			brandName:    "Gym Alpha",
			description:  "Top gym center",
			contactPhone: "0901234567",
			address:      "123 Nguyen Trai, Ha Noi",
			wantErr:      nil,
		},
		{
			name:         "empty id",
			id:           "",
			userID:       "user-1",
			brandName:    "Gym Alpha",
			contactPhone: "0901234567",
			address:      "123 Nguyen Trai",
			wantErr:      derror.ErrInvalidBrandRequest,
		},
		{
			name:         "empty user id",
			id:           "req-1",
			userID:       "",
			brandName:    "Gym Alpha",
			contactPhone: "0901234567",
			address:      "123 Nguyen Trai",
			wantErr:      derror.ErrInvalidBrandRequest,
		},
		{
			name:         "empty brand name",
			id:           "req-1",
			userID:       "user-1",
			brandName:    "   ",
			contactPhone: "0901234567",
			address:      "123 Nguyen Trai",
			wantErr:      derror.ErrInvalidBrandRequest,
		},
		{
			name:         "empty contact phone",
			id:           "req-1",
			userID:       "user-1",
			brandName:    "Gym Alpha",
			contactPhone: "",
			address:      "123 Nguyen Trai",
			wantErr:      derror.ErrInvalidBrandRequest,
		},
		{
			name:         "empty address",
			id:           "req-1",
			userID:       "user-1",
			brandName:    "Gym Alpha",
			contactPhone: "0901234567",
			address:      "",
			wantErr:      derror.ErrInvalidBrandRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewBrandRequest(tt.id, tt.userID, tt.brandName, tt.description, tt.contactPhone, tt.address, now)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("got err %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got, want := req.ID(), tt.id; got != want {
				t.Errorf("got id %v, want %v", got, want)
			}
			if got, want := req.Status(), BrandRequestStatusPending; got != want {
				t.Errorf("got status %v, want %v", got, want)
			}
		})
	}
}

func TestBrandRequest_ApproveAndReject(t *testing.T) {
	now := time.Now()
	req, err := NewBrandRequest("req-1", "user-1", "Gym Alpha", "Desc", "0901234567", "Address", now)
	if err != nil {
		t.Fatalf("failed to create brand request: %v", err)
	}

	// Test Approve
	adminID := "admin-1"
	approveTime := now.Add(time.Hour)
	if err := req.Approve(adminID, approveTime); err != nil {
		t.Fatalf("unexpected error on approve: %v", err)
	}
	if got, want := req.Status(), BrandRequestStatusApproved; got != want {
		t.Errorf("got status %v, want %v", got, want)
	}
	if got, want := req.ReviewedBy(), adminID; got != want {
		t.Errorf("got reviewedBy %v, want %v", got, want)
	}
	if req.ReviewedAt() == nil || !req.ReviewedAt().Equal(approveTime) {
		t.Errorf("got reviewedAt %v, want %v", req.ReviewedAt(), approveTime)
	}

	// Cannot approve again
	if err := req.Approve(adminID, approveTime); err != derror.ErrBrandRequestNotPending {
		t.Errorf("got %v, want %v", err, derror.ErrBrandRequestNotPending)
	}

	// Cannot reject approved request
	if err := req.Reject(adminID, "some reason", approveTime); err != derror.ErrBrandRequestNotPending {
		t.Errorf("got %v, want %v", err, derror.ErrBrandRequestNotPending)
	}

	// New pending request to test Reject
	req2, _ := NewBrandRequest("req-2", "user-2", "Gym Beta", "Desc", "0901234567", "Address", now)
	rejectTime := now.Add(2 * time.Hour)
	if err := req2.Reject(adminID, "Invalid documents", rejectTime); err != nil {
		t.Fatalf("unexpected error on reject: %v", err)
	}
	if got, want := req2.Status(), BrandRequestStatusRejected; got != want {
		t.Errorf("got status %v, want %v", got, want)
	}
	if got, want := req2.RejectionReason(), "Invalid documents"; got != want {
		t.Errorf("got rejectionReason %v, want %v", got, want)
	}
}
