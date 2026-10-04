package entity

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
)

const (
	BrandRequestStatusPending  = "pending"
	BrandRequestStatusApproved = "approved"
	BrandRequestStatusRejected = "rejected"
)

// BrandRequest represents an upgrade request from an end user to become a brand.
type BrandRequest struct {
	id              string
	userID          string
	brandName       string
	description     string
	contactPhone    string
	address         string
	status          string
	rejectionReason string
	reviewedBy      string
	reviewedAt      *time.Time
	createdAt       time.Time
	updatedAt       time.Time
}

// NewBrandRequest validates and creates a new pending BrandRequest entity.
func NewBrandRequest(
	id string,
	userID string,
	brandName string,
	description string,
	contactPhone string,
	address string,
	now time.Time,
) (*BrandRequest, error) {
	if strings.TrimSpace(id) == "" ||
		strings.TrimSpace(userID) == "" ||
		strings.TrimSpace(brandName) == "" ||
		strings.TrimSpace(contactPhone) == "" ||
		strings.TrimSpace(address) == "" {
		return nil, derror.ErrInvalidBrandRequest
	}

	return &BrandRequest{
		id:           id,
		userID:       userID,
		brandName:    strings.TrimSpace(brandName),
		description:  strings.TrimSpace(description),
		contactPhone: strings.TrimSpace(contactPhone),
		address:      strings.TrimSpace(address),
		status:       BrandRequestStatusPending,
		createdAt:    now,
		updatedAt:    now,
	}, nil
}

// ReconstituteBrandRequest reconstitutes a BrandRequest entity from the persistence layer.
func ReconstituteBrandRequest(
	id string,
	userID string,
	brandName string,
	description string,
	contactPhone string,
	address string,
	status string,
	rejectionReason string,
	reviewedBy string,
	reviewedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) *BrandRequest {
	return &BrandRequest{
		id:              id,
		userID:          userID,
		brandName:       brandName,
		description:     description,
		contactPhone:    contactPhone,
		address:         address,
		status:          status,
		rejectionReason: rejectionReason,
		reviewedBy:      reviewedBy,
		reviewedAt:      reviewedAt,
		createdAt:       createdAt,
		updatedAt:       updatedAt,
	}
}

// Approve transitions the brand request from pending to approved.
func (r *BrandRequest) Approve(adminID string, now time.Time) error {
	if r.status != BrandRequestStatusPending {
		return derror.ErrBrandRequestNotPending
	}
	r.status = BrandRequestStatusApproved
	r.reviewedBy = adminID
	reviewedTime := now
	r.reviewedAt = &reviewedTime
	r.updatedAt = now
	return nil
}

// Reject transitions the brand request from pending to rejected with a reason.
func (r *BrandRequest) Reject(adminID string, reason string, now time.Time) error {
	if r.status != BrandRequestStatusPending {
		return derror.ErrBrandRequestNotPending
	}
	r.status = BrandRequestStatusRejected
	r.rejectionReason = strings.TrimSpace(reason)
	r.reviewedBy = adminID
	reviewedTime := now
	r.reviewedAt = &reviewedTime
	r.updatedAt = now
	return nil
}

// Getters
func (r *BrandRequest) ID() string              { return r.id }
func (r *BrandRequest) UserID() string          { return r.userID }
func (r *BrandRequest) BrandName() string       { return r.brandName }
func (r *BrandRequest) Description() string     { return r.description }
func (r *BrandRequest) ContactPhone() string    { return r.contactPhone }
func (r *BrandRequest) Address() string         { return r.address }
func (r *BrandRequest) Status() string          { return r.status }
func (r *BrandRequest) RejectionReason() string { return r.rejectionReason }
func (r *BrandRequest) ReviewedBy() string      { return r.reviewedBy }
func (r *BrandRequest) ReviewedAt() *time.Time  { return r.reviewedAt }
func (r *BrandRequest) CreatedAt() time.Time    { return r.createdAt }
func (r *BrandRequest) UpdatedAt() time.Time    { return r.updatedAt }
