package matchmaker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
)

const (
	RedisQueueKey    = "solo_match:queue:waiting"
	RedisEventPrefix = "solo_match:events:"
)

type RedisMatchmakerAdapter struct {
	redisClient      *redis.Client
	matchRepo        repository.SoloMatchRepository
	botCalc          *service.BotBenchmarkCalculator
	botRepo          repository.BotRepository
	exerciseProvider port.ExerciseProvider
	prProvider       port.UserPRProvider
	pollInterval     time.Duration
	botTimeout       time.Duration
	triggerCh        chan struct{}
	closeCh          chan struct{}
	wg               sync.WaitGroup
}

var _ port.MatchmakerQueuePort = (*RedisMatchmakerAdapter)(nil)

func NewRedisMatchmakerAdapter(
	redisClient *redis.Client,
	matchRepo repository.SoloMatchRepository,
	botCalc *service.BotBenchmarkCalculator,
	botRepo repository.BotRepository,
	exerciseProvider port.ExerciseProvider,
	prProvider port.UserPRProvider,
	pollInterval time.Duration,
	botTimeout time.Duration,
) *RedisMatchmakerAdapter {
	if pollInterval <= 0 {
		pollInterval = 250 * time.Millisecond
	}
	if botTimeout <= 0 {
		botTimeout = 5 * time.Minute
	}
	if botCalc == nil {
		botCalc = service.NewBotBenchmarkCalculator()
	}

	adapter := &RedisMatchmakerAdapter{
		redisClient:      redisClient,
		matchRepo:        matchRepo,
		botCalc:          botCalc,
		botRepo:          botRepo,
		exerciseProvider: exerciseProvider,
		prProvider:       prProvider,
		pollInterval:     pollInterval,
		botTimeout:       botTimeout,
		triggerCh:        make(chan struct{}, 10),
		closeCh:          make(chan struct{}),
	}

	adapter.wg.Add(1)
	go adapter.workerLoop()

	return adapter
}

func (a *RedisMatchmakerAdapter) Enqueue(ctx context.Context, ticket *aggregate.MatchmakingTicket) error {
	member := ticket.ID() + ":" + ticket.UserID()
	score := float64(ticket.JoinedAt().Unix())

	err := a.redisClient.ZAdd(ctx, RedisQueueKey, redis.Z{
		Score:  score,
		Member: member,
	}).Err()
	if err != nil {
		return fmt.Errorf("enqueue redis ticket: %w", err)
	}

	select {
	case a.triggerCh <- struct{}{}:
	default:
	}

	return nil
}

func (a *RedisMatchmakerAdapter) Cancel(ctx context.Context, ticketID, userID string) error {
	member := ticketID + ":" + userID
	err := a.redisClient.ZRem(ctx, RedisQueueKey, member).Err()
	if err != nil {
		return fmt.Errorf("cancel redis ticket: %w", err)
	}
	return nil
}

func (a *RedisMatchmakerAdapter) ListenMatch(ctx context.Context, ticketID string) (<-chan *port.MatchFoundEvent, error) {
	channelName := RedisEventPrefix + ticketID
	pubsub := a.redisClient.Subscribe(ctx, channelName)

	// Kiểm tra kết nối subscribe
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("redis subscribe failed: %w", err)
	}

	outCh := make(chan *port.MatchFoundEvent, 1)

	go func() {
		defer func() {
			_ = pubsub.Close()
			close(outCh)
		}()

		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case <-a.closeCh:
				return
			case msg, ok := <-ch:
				if !ok {
					// Redis connection mất hoặc PubSub đóng -> Thoát để kích hoạt fail-fast
					return
				}
				matchID := msg.Payload
				if matchID != "" {
					match, err := a.matchRepo.FindByID(ctx, matchID)
					if err == nil && match != nil {
						outCh <- &port.MatchFoundEvent{
							MatchID: matchID,
							Match:   match,
						}
						return
					}
				}
			}
		}
	}()

	return outCh, nil
}

func (a *RedisMatchmakerAdapter) Close() error {
	close(a.closeCh)
	a.wg.Wait()
	return nil
}

func (a *RedisMatchmakerAdapter) workerLoop() {
	defer a.wg.Done()
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.closeCh:
			return
		case <-a.triggerCh:
			a.matchPairs()
		case <-ticker.C:
			a.matchPairs()
			a.fallbackBots()
		}
	}
}

func (a *RedisMatchmakerAdapter) matchPairs() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Lua script nguyên tử: Lấy 2 phần tử đầu tiên và xóa khỏi ZSET nếu đủ 2 phần tử
	luaScript := `
		local members = redis.call('ZRANGE', KEYS[1], 0, 1)
		if #members >= 2 then
			redis.call('ZREM', KEYS[1], members[1], members[2])
			return members
		end
		return {}
	`

	res, err := a.redisClient.Eval(ctx, luaScript, []string{RedisQueueKey}).Result()
	if err != nil {
		log.Printf("[SoloMatch Matchmaker] Redis eval error in matchPairs: %v", err)
		return
	}

	members, ok := res.([]interface{})
	if !ok || len(members) < 2 {
		return
	}

	m1Str := fmt.Sprintf("%v", members[0])
	m2Str := fmt.Sprintf("%v", members[1])

	t1Parts := strings.Split(m1Str, ":")
	t2Parts := strings.Split(m2Str, ":")
	if len(t1Parts) != 2 || len(t2Parts) != 2 {
		log.Printf("[SoloMatch Matchmaker] Invalid queue member format: m1=%q, m2=%q", m1Str, m2Str)
		return
	}

	t1ID, u1ID := t1Parts[0], t1Parts[1]
	t2ID, u2ID := t2Parts[0], t2Parts[1]

	matchID := uuid.New().String()
	now := time.Now().UTC()

	p1, err := entity.NewMatchParticipant(uuid.New().String(), matchID, u1ID)
	if err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to create participant 1 (%s): %v", u1ID, err)
		return
	}
	p2, err := entity.NewMatchParticipant(uuid.New().String(), matchID, u2ID)
	if err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to create participant 2 (%s): %v", u2ID, err)
		return
	}

	challenge, err := a.exerciseProvider.GetRandomChallenge(ctx)
	if err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to get exercise challenge for match %s: %v", matchID, err)
		return
	}

	match, err := aggregate.NewSoloMatch(
		matchID,
		challenge.ExerciseID,
		challenge.ExerciseName,
		challenge.ChallengeType,
		challenge.Duration,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to create PvP match %s: %v", matchID, err)
		return
	}

	// Lưu match vào PostgreSQL
	if err := a.matchRepo.Save(ctx, match); err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to save PvP match %s to database: %v", matchID, err)
		return
	}

	// Publish thông báo qua Redis PubSub cho cả 2 vé
	if err := a.redisClient.Publish(ctx, RedisEventPrefix+t1ID, matchID).Err(); err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to publish match %s to ticket %s: %v", matchID, t1ID, err)
	}
	if err := a.redisClient.Publish(ctx, RedisEventPrefix+t2ID, matchID).Err(); err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to publish match %s to ticket %s: %v", matchID, t2ID, err)
	}
}

func (a *RedisMatchmakerAdapter) fallbackBots() {
	if a.botRepo == nil {
		return // Bot matching is disabled
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	threshold := float64(time.Now().UTC().Add(-a.botTimeout).Unix())

	// Lua script: Lấy 1 vé có score <= threshold và xóa khỏi ZSET
	luaScript := `
		local members = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, 1)
		if #members >= 1 then
			redis.call('ZREM', KEYS[1], members[1])
			return members[1]
		end
		return ""
	`

	res, err := a.redisClient.Eval(ctx, luaScript, []string{RedisQueueKey}, threshold).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		log.Printf("[SoloMatch Matchmaker] Redis eval error in fallbackBots: %v", err)
		return
	}
	if res == nil || res == "" {
		return
	}

	memberStr := fmt.Sprintf("%v", res)
	parts := strings.Split(memberStr, ":")
	if len(parts) != 2 {
		log.Printf("[SoloMatch Matchmaker] Invalid bot queue member format: %q", memberStr)
		return
	}

	ticketID, userID := parts[0], parts[1]
	matchID := uuid.New().String()
	now := time.Now().UTC()

	pUser, err := entity.NewMatchParticipant(uuid.New().String(), matchID, userID)
	if err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to create user participant for bot match %s: %v", matchID, err)
		return
	}
	var botUserID string
	if a.botRepo != nil {
		id, bErr := a.botRepo.GetRandomBotUserID(ctx)
		if bErr == nil {
			botUserID = id
		} else {
			log.Printf("[SoloMatch Matchmaker] Failed to get random bot user id: %v", bErr)
		}
	}
	if botUserID == "" {
		log.Printf("[SoloMatch Matchmaker] No bot available to match with ticket %s (user %s)", ticketID, userID)
		return
	}
	pBot, err := entity.NewMatchParticipant(uuid.New().String(), matchID, botUserID)
	if err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to create bot participant for match %s: %v", matchID, err)
		return
	}

	challenge, err := a.exerciseProvider.GetRandomChallenge(ctx)
	if err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to get exercise challenge for bot match %s: %v", matchID, err)
		return
	}

	var userPR *float64
	if a.prProvider != nil {
		pr, prErr := a.prProvider.GetBestPR(ctx, userID, challenge.ExerciseID)
		if prErr != nil {
			log.Printf("[SoloMatch Matchmaker] Failed to get user PR for user %s on exercise %s: %v", userID, challenge.ExerciseID, prErr)
		} else {
			userPR = pr
		}
	}

	botScore := a.botCalc.CalculateBotScore(challenge.ChallengeType, userPR)
	_ = pBot.RecordResult(botScore, now)

	match, err := aggregate.NewSoloMatch(
		matchID,
		challenge.ExerciseID,
		challenge.ExerciseName,
		challenge.ChallengeType,
		challenge.Duration,
		[]*entity.MatchParticipant{pUser, pBot},
		now,
	)
	if err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to create bot match aggregate %s: %v", matchID, err)
		return
	}

	if err := a.matchRepo.Save(ctx, match); err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to save bot match %s to database: %v", matchID, err)
		return
	}

	if err := a.redisClient.Publish(ctx, RedisEventPrefix+ticketID, matchID).Err(); err != nil {
		log.Printf("[SoloMatch Matchmaker] Failed to publish bot match %s to ticket %s: %v", matchID, ticketID, err)
	}
}
