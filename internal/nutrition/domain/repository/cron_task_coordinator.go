package repository

import (
	"context"
	"time"
)

// CronTaskCoordinator defines the domain port for distributed coordination of recurring background cron jobs.
// It coordinates multi-node batch execution through shared state, atomic initialization, and in-flight locking.
type CronTaskCoordinator interface {
	// EnsureTaskSet guarantees that the task set for the given job and target date is loaded into the shared coordinator.
	// If the task set has not yet been initialized, one node atomically wins the init lock and calls loaderFn to populate it.
	// Other nodes wait briefly for the winner to finish populating and retrieve the populated task set.
	EnsureTaskSet(ctx context.Context, jobName, targetDate string, loaderFn func() ([]string, error)) ([]string, error)

	// TryClaimUser attempts to atomically claim execution rights for a specific user within the active task set.
	// Returns true if the user was successfully claimed, or false if another node is currently processing or has finished it.
	TryClaimUser(ctx context.Context, jobName, targetDate, userID string, ttl time.Duration) (bool, error)

	// CompleteUser marks a user's task as finished by removing the user from the task set and releasing any held lock.
	CompleteUser(ctx context.Context, jobName, targetDate, userID string) error

	// ReleaseUserLock releases the in-flight lock for a user without removing them from the task set, allowing future retry.
	ReleaseUserLock(ctx context.Context, jobName, targetDate, userID string) error
}
