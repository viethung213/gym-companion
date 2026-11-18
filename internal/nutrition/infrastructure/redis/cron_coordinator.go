package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

var _ repository.CronTaskCoordinator = (*CronTaskCoordinator)(nil)

// CronTaskCoordinator implements repository.CronTaskCoordinator using Redis.
type CronTaskCoordinator struct {
	client       *redis.Client
	nodeID       string
	pollInterval time.Duration
	pollTimeout  time.Duration
}

// NewCronTaskCoordinator creates a new CronTaskCoordinator.
func NewCronTaskCoordinator(client *redis.Client) *CronTaskCoordinator {
	hostname, _ := os.Hostname()
	nodeID := fmt.Sprintf("%s-%s", hostname, uuid.NewString()[:8])

	return &CronTaskCoordinator{
		client:       client,
		nodeID:       nodeID,
		pollInterval: 200 * time.Millisecond,
		pollTimeout:  10 * time.Second,
	}
}

// WithPollSettings configures polling behavior for waiting nodes.
func (c *CronTaskCoordinator) WithPollSettings(interval, timeout time.Duration) *CronTaskCoordinator {
	c.pollInterval = interval
	c.pollTimeout = timeout
	return c
}

func (c *CronTaskCoordinator) taskKey(jobName, targetDate string) string {
	return fmt.Sprintf("nutrition:cron:tasks:%s:%s", jobName, targetDate)
}

func (c *CronTaskCoordinator) emptyKey(jobName, targetDate string) string {
	return fmt.Sprintf("nutrition:cron:empty:%s:%s", jobName, targetDate)
}

func (c *CronTaskCoordinator) initKey(jobName, targetDate string) string {
	return fmt.Sprintf("nutrition:cron:init:%s:%s", jobName, targetDate)
}

func (c *CronTaskCoordinator) lockKey(jobName, targetDate, userID string) string {
	return fmt.Sprintf("nutrition:cron:lock:%s:%s:%s", jobName, targetDate, userID)
}

// EnsureTaskSet guarantees the task set is loaded on Redis.
// Only one node queries the DB via loaderFn; other nodes wait and load from Redis.
func (c *CronTaskCoordinator) EnsureTaskSet(ctx context.Context, jobName, targetDate string, loaderFn func() ([]string, error)) ([]string, error) {
	if c.client == nil {
		return nil, errors.New("redis client is nil in CronTaskCoordinator")
	}

	taskKey := c.taskKey(jobName, targetDate)
	emptyKey := c.emptyKey(jobName, targetDate)
	initKey := c.initKey(jobName, targetDate)

	// 1. Fast path: check if already initialized as empty
	emptyExists, err := c.client.Exists(ctx, emptyKey).Result()
	if err != nil {
		return nil, fmt.Errorf("check empty key: %w", err)
	}
	if emptyExists > 0 {
		return []string{}, nil
	}

	// 2. Fast path: check if task set already exists
	taskExists, err := c.client.Exists(ctx, taskKey).Result()
	if err != nil {
		return nil, fmt.Errorf("check task key: %w", err)
	}
	if taskExists > 0 {
		return c.client.SMembers(ctx, taskKey).Result()
	}

	// 3. Neither exists: race for the init lock
	acquired, err := c.client.SetNX(ctx, initKey, c.nodeID, 60*time.Second).Result()
	if err != nil {
		return nil, fmt.Errorf("acquire init lock: %w", err)
	}

	if acquired {
		// Winner node: execute loaderFn and populate Redis
		userIDs, loadErr := loaderFn()
		if loadErr != nil {
			_ = c.client.Del(ctx, initKey).Err()
			return nil, fmt.Errorf("loader function failed: %w", loadErr)
		}

		if len(userIDs) == 0 {
			if setErr := c.client.Set(ctx, emptyKey, "1", 24*time.Hour).Err(); setErr != nil {
				return nil, fmt.Errorf("set empty key: %w", setErr)
			}
			return []string{}, nil
		}

		pipe := c.client.Pipeline()
		members := make([]interface{}, len(userIDs))
		for i, id := range userIDs {
			members[i] = id
		}
		pipe.SAdd(ctx, taskKey, members...)
		pipe.Expire(ctx, taskKey, 24*time.Hour)
		if _, execErr := pipe.Exec(ctx); execErr != nil {
			_ = c.client.Del(ctx, initKey).Err()
			return nil, fmt.Errorf("populate task set: %w", execErr)
		}

		return userIDs, nil
	}

	// 4. Loser node: wait for the winner to finish populating
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	timeoutChan := time.After(c.pollTimeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutChan:
			return nil, fmt.Errorf("timeout waiting for task set initialization for job %s on %s", jobName, targetDate)
		case <-ticker.C:
			// Check if populated as empty
			if c.client.Exists(ctx, emptyKey).Val() > 0 {
				return []string{}, nil
			}
			// Check if populated with tasks
			if c.client.Exists(ctx, taskKey).Val() > 0 {
				return c.client.SMembers(ctx, taskKey).Result()
			}
		}
	}
}

// TryClaimUser attempts to atomically claim execution rights for a specific user.
func (c *CronTaskCoordinator) TryClaimUser(ctx context.Context, jobName, targetDate, userID string, ttl time.Duration) (bool, error) {
	if c.client == nil {
		return false, errors.New("redis client is nil in CronTaskCoordinator")
	}

	taskKey := c.taskKey(jobName, targetDate)
	lockKey := c.lockKey(jobName, targetDate, userID)

	// Verify user is still in the active task set (not completed by someone else)
	isMember, err := c.client.SIsMember(ctx, taskKey, userID).Result()
	if err != nil {
		return false, fmt.Errorf("check task membership: %w", err)
	}
	if !isMember {
		return false, nil
	}

	// Attempt in-flight lock
	acquired, err := c.client.SetNX(ctx, lockKey, c.nodeID, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire in-flight lock: %w", err)
	}

	return acquired, nil
}

// CompleteUser marks the user as finished by removing from the task set and releasing the lock.
func (c *CronTaskCoordinator) CompleteUser(ctx context.Context, jobName, targetDate, userID string) error {
	if c.client == nil {
		return errors.New("redis client is nil in CronTaskCoordinator")
	}

	taskKey := c.taskKey(jobName, targetDate)
	lockKey := c.lockKey(jobName, targetDate, userID)

	pipe := c.client.Pipeline()
	pipe.SRem(ctx, taskKey, userID)
	pipe.Del(ctx, lockKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("complete user task: %w", err)
	}
	return nil
}

// ReleaseUserLock releases the in-flight lock without removing the user from the task set (allowing retry).
func (c *CronTaskCoordinator) ReleaseUserLock(ctx context.Context, jobName, targetDate, userID string) error {
	if c.client == nil {
		return errors.New("redis client is nil in CronTaskCoordinator")
	}

	lockKey := c.lockKey(jobName, targetDate, userID)
	return c.client.Del(ctx, lockKey).Err()
}
