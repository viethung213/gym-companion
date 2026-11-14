package matchmaker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
)

const (
	RedisQueueKey    = "solo_match:queue:waiting"
	RedisEventPrefix = "solo_match:events:"
)

type RedisMatchmakerAdapter struct {
	redisClient      *redis.Client
	matchRepo        repository.SoloMatchRepository
	botRepo          repository.BotRepository
	exerciseProvider port.ExerciseProvider
	eventRecorder    port.EventRecorder
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
	botRepo repository.BotRepository,
	exerciseProvider port.ExerciseProvider,
	eventRecorder port.EventRecorder,
	pollInterval time.Duration,
	botTimeout time.Duration,
) *RedisMatchmakerAdapter {
	if pollInterval <= 0 {
		pollInterval = 250 * time.Millisecond
	}
	if botTimeout <= 0 {
		botTimeout = 5 * time.Minute
	}

	adapter := &RedisMatchmakerAdapter{
		redisClient:      redisClient,
		matchRepo:        matchRepo,
		botRepo:          botRepo,
		exerciseProvider: exerciseProvider,
		eventRecorder:    eventRecorder,
		pollInterval:     pollInterval,
		botTimeout:       botTimeout,
		triggerCh:        make(chan struct{}, 10),
		closeCh:          make(chan struct{}),
	}

	adapter.wg.Add(1)
	go adapter.workerLoop()

	return adapter
}

func (a *RedisMatchmakerAdapter) IsUserInQueue(ctx context.Context, userID string) (bool, error) {
	luaScript := `
		local members = redis.call('ZRANGE', KEYS[1], 0, -1)
		for _, m in ipairs(members) do
			local u = string.match(m, ":([^:]+)$")
			if u == ARGV[1] then
				return 1
			end
		end
		return 0
	`
	res, err := a.redisClient.Eval(ctx, luaScript, []string{RedisQueueKey}, userID).Int()
	if err != nil {
		return false, fmt.Errorf("check user in redis queue: %w", err)
	}
	return res == 1, nil
}

func (a *RedisMatchmakerAdapter) Enqueue(ctx context.Context, ticket *aggregate.MatchmakingTicket) error {
	member := ticket.ID() + ":" + ticket.UserID()
	score := float64(ticket.JoinedAt().Unix())

	luaScript := `
		local members = redis.call('ZRANGE', KEYS[1], 0, -1)
		for _, m in ipairs(members) do
			local u = string.match(m, ":([^:]+)$")
			if u == ARGV[2] then
				return 0
			end
		end
		redis.call('ZADD', KEYS[1], ARGV[1], ARGV[3])
		return 1
	`
	res, err := a.redisClient.Eval(ctx, luaScript, []string{RedisQueueKey}, score, ticket.UserID(), member).Int()
	if err != nil {
		return fmt.Errorf("enqueue redis ticket: %w", err)
	}
	if res == 0 {
		return derror.ErrUserAlreadyInQueue
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
		if cErr := pubsub.Close(); cErr != nil {
			slog.ErrorContext(ctx, "Error closing pubsub", slog.Any("error", cErr))
		}
		return nil, fmt.Errorf("redis subscribe failed: %w", err)
	}

	outCh := make(chan *port.MatchFoundEvent, 1)

	go func() {
		defer func() {
			if cErr := pubsub.Close(); cErr != nil {
				slog.ErrorContext(ctx, "Error closing pubsub", slog.Any("error", cErr))
			}
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
				payload := msg.Payload
				if strings.HasPrefix(payload, "ERROR:") {
					errMsg := strings.TrimPrefix(payload, "ERROR:")
					outCh <- &port.MatchFoundEvent{
						Err: fmt.Errorf("matchmaking failed: %s", errMsg),
					}
					return
				}
				matchID := payload
				if matchID == "" {
					outCh <- &port.MatchFoundEvent{
						Err: errors.New("matchmaking received empty match id"),
					}
					return
				}

				match, err := a.matchRepo.FindByID(ctx, matchID)
				if err != nil {
					outCh <- &port.MatchFoundEvent{
						Err: fmt.Errorf("find match %s: %w", matchID, err),
					}
					return
				}
				if match == nil {
					outCh <- &port.MatchFoundEvent{
						Err: fmt.Errorf("match %s not found in repository", matchID),
					}
					return
				}

				outCh <- &port.MatchFoundEvent{
					MatchID: matchID,
					Match:   match,
				}
				return
			}
		}
	}()

	return outCh, nil
}

func (a *RedisMatchmakerAdapter) notifyTicketsError(ctx context.Context, ticketIDs []string, errMsg string) {
	for _, ticketID := range ticketIDs {
		if ticketID == "" {
			continue
		}
		if err := a.redisClient.Publish(ctx, RedisEventPrefix+ticketID, "ERROR:"+errMsg).Err(); err != nil {
			slog.ErrorContext(ctx, "Failed to publish error to ticket", slog.String("ticket_id", ticketID), slog.Any("error", err))
		}
	}
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

	// Lua script nguyên tử: Lấy 2 phần tử đầu tiên có user_id khác nhau và xóa khỏi ZSET
	luaScript := `
		local members = redis.call('ZRANGE', KEYS[1], 0, -1)
		if #members < 2 then
			return {}
		end

		local first = members[1]
		local first_user = string.match(first, ":([^:]+)$")

		for i = 2, #members do
			local second = members[i]
			local second_user = string.match(second, ":([^:]+)$")
			if first_user ~= second_user then
				redis.call('ZREM', KEYS[1], first, second)
				return {first, second}
			end
		end

		return {}
	`

	res, err := a.redisClient.Eval(ctx, luaScript, []string{RedisQueueKey}).Result()
	if err != nil {
		slog.ErrorContext(ctx, "Redis eval error in matchPairs", slog.Any("error", err))
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
		slog.ErrorContext(ctx, "Invalid queue member format", slog.String("m1", m1Str), slog.String("m2", m2Str))
		return
	}

	t1ID, u1ID := t1Parts[0], t1Parts[1]
	t2ID, u2ID := t2Parts[0], t2Parts[1]

	if u1ID == u2ID {
		slog.WarnContext(ctx, "Refusing to match identical users", slog.String("u1", u1ID), slog.String("u2", u2ID), slog.String("t1", t1ID), slog.String("t2", t2ID))
		a.notifyTicketsError(ctx, []string{t1ID, t2ID}, "cannot match user with themselves")
		return
	}

	matchID := uuid.New().String()
	now := time.Now().UTC()

	p1, err := entity.NewMatchParticipant(uuid.New().String(), matchID, u1ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create participant 1", slog.String("user_id", u1ID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{t1ID, t2ID}, "failed to create match participant")
		return
	}
	p2, err := entity.NewMatchParticipant(uuid.New().String(), matchID, u2ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create participant 2", slog.String("user_id", u2ID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{t1ID, t2ID}, "failed to create match participant")
		return
	}

	challenge, err := a.exerciseProvider.GetRandomChallenge(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get exercise challenge for match", slog.String("match_id", matchID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{t1ID, t2ID}, "failed to get exercise challenge")
		return
	}

	match, err := aggregate.NewSoloMatch(
		matchID,
		challenge.ExerciseID,
		challenge.ExerciseName,
		challenge.ChallengeType,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create PvP match", slog.String("match_id", matchID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{t1ID, t2ID}, "failed to initialize match aggregate")
		return
	}

	// Lưu match vào PostgreSQL
	if err := a.matchRepo.Save(ctx, match); err != nil {
		slog.ErrorContext(ctx, "Failed to save PvP match to database", slog.String("match_id", matchID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{t1ID, t2ID}, "failed to save match to database")
		return
	}

	startEvt := &event.SoloMatchStarted{
		MatchID:       matchID,
		ExerciseID:    challenge.ExerciseID,
		ExerciseName:  challenge.ExerciseName,
		ChallengeType: challenge.ChallengeType.String(),
		Player1ID:     u1ID,
		Player2ID:     u2ID,
		BotUserID:     "",
		StartedAt:     now,
	}
	if a.eventRecorder != nil {
		if pubErr := a.eventRecorder.RecordEvents(ctx, []any{startEvt}); pubErr != nil {
			slog.ErrorContext(ctx, "Failed to record SoloMatchStarted event for PvP", slog.String("match_id", matchID), slog.Any("error", pubErr))
		}
	}

	// Publish thông báo qua Redis PubSub cho cả 2 vé
	if err := a.redisClient.Publish(ctx, RedisEventPrefix+t1ID, matchID).Err(); err != nil {
		slog.ErrorContext(ctx, "Failed to publish match to ticket", slog.String("match_id", matchID), slog.String("ticket_id", t1ID), slog.Any("error", err))
	}
	if err := a.redisClient.Publish(ctx, RedisEventPrefix+t2ID, matchID).Err(); err != nil {
		slog.ErrorContext(ctx, "Failed to publish match to ticket", slog.String("match_id", matchID), slog.String("ticket_id", t2ID), slog.Any("error", err))
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
		slog.ErrorContext(ctx, "Redis eval error in fallbackBots", slog.Any("error", err))
		return
	}
	if res == nil || res == "" {
		return
	}

	memberStr := fmt.Sprintf("%v", res)
	parts := strings.Split(memberStr, ":")
	if len(parts) != 2 {
		slog.ErrorContext(ctx, "Invalid bot queue member format", slog.String("member", memberStr))
		return
	}

	ticketID, userID := parts[0], parts[1]
	matchID := uuid.New().String()
	now := time.Now().UTC()

	pUser, err := entity.NewMatchParticipant(uuid.New().String(), matchID, userID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create user participant for bot match", slog.String("match_id", matchID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{ticketID}, "failed to create user participant")
		return
	}

	var botUserID string
	if a.botRepo != nil {
		id, bErr := a.botRepo.GetRandomBotUserID(ctx)
		if bErr == nil {
			botUserID = id
		} else {
			slog.ErrorContext(ctx, "Failed to get random bot user id", slog.Any("error", bErr))
		}
	}
	if botUserID == "" {
		slog.WarnContext(ctx, "No bot available to match with ticket", slog.String("ticket_id", ticketID), slog.String("user_id", userID))
		a.notifyTicketsError(ctx, []string{ticketID}, "no bot available for match")
		return
	}
	if userID == botUserID {
		slog.WarnContext(ctx, "User has identical id as bot", slog.String("user_id", userID), slog.String("bot_id", botUserID))
		a.notifyTicketsError(ctx, []string{ticketID}, "user and bot have identical id")
		return
	}

	pBot, err := entity.NewMatchParticipant(uuid.New().String(), matchID, botUserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create bot participant for match", slog.String("match_id", matchID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{ticketID}, "failed to create bot participant")
		return
	}

	challenge, err := a.exerciseProvider.GetRandomChallenge(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get exercise challenge for bot match", slog.String("match_id", matchID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{ticketID}, "failed to get exercise challenge")
		return
	}

	match, err := aggregate.NewSoloMatch(
		matchID,
		challenge.ExerciseID,
		challenge.ExerciseName,
		challenge.ChallengeType,
		[]*entity.MatchParticipant{pUser, pBot},
		now,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create bot match aggregate", slog.String("match_id", matchID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{ticketID}, "failed to initialize bot match aggregate")
		return
	}

	if err := a.matchRepo.Save(ctx, match); err != nil {
		slog.ErrorContext(ctx, "Failed to save bot match to database", slog.String("match_id", matchID), slog.Any("error", err))
		a.notifyTicketsError(ctx, []string{ticketID}, "failed to save bot match to database")
		return
	}

	startEvt := &event.SoloMatchStarted{
		MatchID:       matchID,
		ExerciseID:    challenge.ExerciseID,
		ExerciseName:  challenge.ExerciseName,
		ChallengeType: challenge.ChallengeType.String(),
		Player1ID:     userID,
		Player2ID:     botUserID,
		BotUserID:     botUserID,
		StartedAt:     now,
	}
	if a.eventRecorder != nil {
		if pubErr := a.eventRecorder.RecordEvents(ctx, []any{startEvt}); pubErr != nil {
			slog.ErrorContext(ctx, "Failed to record SoloMatchStarted event for bot match", slog.String("match_id", matchID), slog.Any("error", pubErr))
			a.notifyTicketsError(ctx, []string{ticketID}, "failed to initialize bot match event")
			return
		}
	}

	if err := a.redisClient.Publish(ctx, RedisEventPrefix+ticketID, matchID).Err(); err != nil {
		slog.ErrorContext(ctx, "Failed to publish bot match to ticket", slog.String("match_id", matchID), slog.String("ticket_id", ticketID), slog.Any("error", err))
	}
}
