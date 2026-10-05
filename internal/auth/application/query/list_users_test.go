package query

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

type mockQueryUserRepo struct {
	users []*aggregate.User
}

func (m *mockQueryUserRepo) Create(ctx context.Context, user *aggregate.User) error {
	m.users = append(m.users, user)
	return nil
}

func (m *mockQueryUserRepo) Update(ctx context.Context, user *aggregate.User) error {
	return nil
}

func (m *mockQueryUserRepo) FindByID(ctx context.Context, id string) (*aggregate.User, error) {
	return nil, nil
}

func (m *mockQueryUserRepo) FindByIdentity(ctx context.Context, identityType string, identifier string) (*aggregate.User, error) {
	return nil, nil
}

func (m *mockQueryUserRepo) List(ctx context.Context, filter repository.ListUsersFilter) ([]*aggregate.User, int, error) {
	var filtered []*aggregate.User
	for _, u := range m.users {
		if filter.Role != "" && !strings.EqualFold(u.Role(), filter.Role) {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(u.Status(), filter.Status) {
			continue
		}
		if filter.Search != "" {
			term := strings.ToLower(filter.Search)
			nameMatch := strings.Contains(strings.ToLower(u.FullName()), term)
			identMatch := strings.Contains(strings.ToLower(u.Identity().Identifier()), term)
			if !nameMatch && !identMatch {
				continue
			}
		}
		filtered = append(filtered, u)
	}

	total := len(filtered)
	start := filter.Offset
	if start > total {
		start = total
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}

	return filtered[start:end], total, nil
}

func TestListUsersHandler(t *testing.T) {
	ctx := context.Background()

	roleUser, _ := vo.NewRole(vo.RoleUser)
	roleBrand, _ := vo.NewRole(vo.RoleBrand)
	roleAdmin, _ := vo.NewRole(vo.RoleAdmin)

	u1 := aggregate.NewUser("u-1", "Nguyen Van A", roleUser, aggregate.UserStatusActive, aggregate.NewIdentity("id-1", "email", "vana@example.com", "", nil, time.Now(), time.Now()), time.Now(), time.Now())
	u2 := aggregate.NewUser("u-2", "Gym Brand B", roleBrand, aggregate.UserStatusActive, aggregate.NewIdentity("id-2", "phone", "0901234567", "", nil, time.Now(), time.Now()), time.Now(), time.Now())
	u3 := aggregate.NewUser("u-3", "Admin Master", roleAdmin, aggregate.UserStatusActive, aggregate.NewIdentity("id-3", "email", "admin@gym.com", "", nil, time.Now(), time.Now()), time.Now(), time.Now())
	u4 := aggregate.NewUser("u-4", "Locked User", roleUser, aggregate.UserStatusLocked, aggregate.NewIdentity("id-4", "email", "locked@example.com", "", nil, time.Now(), time.Now()), time.Now(), time.Now())

	repo := &mockQueryUserRepo{
		users: []*aggregate.User{u1, u2, u3, u4},
	}
	handler := NewListUsersHandler(repo)

	t.Run("list all default pagination", func(t *testing.T) {
		res, err := handler.Handle(ctx, ListUsersQuery{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 4 {
			t.Errorf("got total %d, want 4", res.Total)
		}
		if len(res.Items) != 4 {
			t.Errorf("got len %d, want 4", len(res.Items))
		}
	})

	t.Run("filter by role", func(t *testing.T) {
		res, err := handler.Handle(ctx, ListUsersQuery{Role: "brand"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("got total %d, want 1", res.Total)
		}
		if res.Items[0].FullName() != "Gym Brand B" {
			t.Errorf("got name %s, want Gym Brand B", res.Items[0].FullName())
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		res, err := handler.Handle(ctx, ListUsersQuery{Status: "locked"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("got total %d, want 1", res.Total)
		}
		if res.Items[0].ID() != "u-4" {
			t.Errorf("got id %s, want u-4", res.Items[0].ID())
		}
	})

	t.Run("search by name keyword", func(t *testing.T) {
		res, err := handler.Handle(ctx, ListUsersQuery{Search: "Van A"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("got total %d, want 1", res.Total)
		}
		if res.Items[0].ID() != "u-1" {
			t.Errorf("got id %s, want u-1", res.Items[0].ID())
		}
	})

	t.Run("search by phone keyword", func(t *testing.T) {
		res, err := handler.Handle(ctx, ListUsersQuery{Search: "0901234567"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("got total %d, want 1", res.Total)
		}
		if res.Items[0].ID() != "u-2" {
			t.Errorf("got id %s, want u-2", res.Items[0].ID())
		}
	})

	t.Run("pagination limit", func(t *testing.T) {
		res, err := handler.Handle(ctx, ListUsersQuery{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 4 {
			t.Errorf("got total %d, want 4", res.Total)
		}
		if len(res.Items) != 2 {
			t.Errorf("got items len %d, want 2", len(res.Items))
		}
	})
}
