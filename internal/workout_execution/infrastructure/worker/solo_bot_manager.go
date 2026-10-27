package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

const (
	DefaultBotReps            int32 = 5
	DefaultBotDurationSeconds int32 = 300 // 5 minutes

	defaultRepInterval = 2500 * time.Millisecond
	defaultDurInterval = 1000 * time.Millisecond
)

var _ port.SoloBotManager = (*SoloBotManager)(nil)

// SoloBotManager manages virtual bot lifecycles and background simulation runners.
type SoloBotManager struct {
	prRepo   repository.PersonalRecordRepository
	liveSync port.SoloMatchLiveSyncPort
	outbox   port.OutboxWriter

	mu           sync.Mutex
	repInterval  time.Duration
	durInterval  time.Duration
	botCancelers map[string]context.CancelFunc
	botWg        sync.WaitGroup
}

// NewSoloBotManager constructs a new SoloBotManager instance.
func NewSoloBotManager(
	prRepo repository.PersonalRecordRepository,
	liveSync port.SoloMatchLiveSyncPort,
	outbox port.OutboxWriter,
) *SoloBotManager {
	return &SoloBotManager{
		prRepo:       prRepo,
		liveSync:     liveSync,
		outbox:       outbox,
		repInterval:  defaultRepInterval,
		durInterval:  defaultDurInterval,
		botCancelers: make(map[string]context.CancelFunc),
	}
}

// StartBot initializes and launches a virtual bot simulation runner.
func (m *SoloBotManager) StartBot(
	ctx context.Context,
	matchID, botUserID, userPlayerID, exerciseID, challengeType string,
) error {
	if botUserID == "" || matchID == "" {
		return nil
	}

	chType := parseChallengeType(challengeType)
	botScore := m.resolveBotScore(ctx, userPlayerID, exerciseID, chType)

	// Giai đoạn 1: Đăng ký Bot vào LiveSync ở trạng thái WAITING (chưa đếm số)
	if m.liveSync != nil {
		if regErr := m.liveSync.RegisterParticipant(ctx, matchID, botUserID, exerciseID); regErr != nil {
			return fmt.Errorf("register bot to live sync: %w", regErr)
		}
		botWaiting := port.ParticipantProgress{
			UserID:    botUserID,
			Value:     0,
			FormScore: 100.0,
			Status:    "WAITING",
		}
		if pubErr := m.liveSync.PublishMyProgress(ctx, matchID, botWaiting); pubErr != nil {
			log.Printf("[SoloBotManager] Warning: failed to publish bot waiting progress: %v", pubErr)
		}
	}

	// Giai đoạn 2 & 3 & 4: Khởi chạy Virtual Bot Runner để mô phỏng vòng đời người thật
	botTimeout := 6 * time.Minute
	if chType == vo.ChallengeTypeDuration {
		botTimeout = 16 * time.Minute
	}
	runnerCtx, cancel := context.WithTimeout(ctx, botTimeout)

	m.mu.Lock()
	m.botCancelers[matchID] = cancel
	m.mu.Unlock()

	m.botWg.Add(1)
	go func() {
		defer m.botWg.Done()
		defer cancel()
		defer func() {
			m.mu.Lock()
			delete(m.botCancelers, matchID)
			m.mu.Unlock()
		}()
		m.runVirtualBot(runnerCtx, matchID, botUserID, exerciseID, chType, botScore)
	}()

	return nil
}

// CancelBot cancels the running virtual bot goroutine for the specified match.
func (m *SoloBotManager) CancelBot(matchID string) {
	m.mu.Lock()
	cancel, exists := m.botCancelers[matchID]
	if exists {
		delete(m.botCancelers, matchID)
	}
	m.mu.Unlock()

	if exists && cancel != nil {
		log.Printf("[SoloBotManager] CancelBot invoked for match %s", matchID)
		cancel()
	}
}

// StopAll cancels all active virtual bot runners and blocks until all goroutines exit cleanly.
func (m *SoloBotManager) StopAll() {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.botCancelers))
	for matchID, cancel := range m.botCancelers {
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
		delete(m.botCancelers, matchID)
	}
	m.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	m.botWg.Wait()
	log.Println("[SoloBotManager] All virtual bot runners stopped cleanly.")
}

// SetRunnerDelaysForTesting allows test suites to customize bot timing for fast execution.
func (m *SoloBotManager) SetRunnerDelaysForTesting(repInterval, durInterval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repInterval = repInterval
	m.durInterval = durInterval
}

func (m *SoloBotManager) getIntervals() (repInterval, durInterval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.repInterval, m.durInterval
}

func (m *SoloBotManager) runVirtualBot(
	ctx context.Context,
	matchID, botUserID, exerciseID string,
	chType vo.ChallengeType,
	botScore int32,
) {
	if m.liveSync == nil {
		return
	}

	repInterval, durInterval := m.getIntervals()

	defer func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if unregErr := m.liveSync.UnregisterParticipant(cleanCtx, matchID, botUserID); unregErr != nil {
			log.Printf("[SoloBotManager] Virtual bot failed to unregister participant %s for match %s: %v", botUserID, matchID, unregErr)
		}
	}()

	oppUpdates, unsub, err := m.liveSync.SubscribeMatchUpdates(ctx, matchID, botUserID)
	if err != nil {
		log.Printf("[SoloBotManager] Virtual bot failed to subscribe match updates for %s: %v", matchID, err)
		return
	}
	defer unsub()

	// Giai đoạn 1: Chờ người thật vào phòng (chờ cho tới khi phòng sẵn sàng hoặc context bị hủy)
	pollTicker := time.NewTicker(200 * time.Millisecond)
	defer pollTicker.Stop()

	var isActivated bool
	for !isActivated {
		select {
		case <-ctx.Done():
			log.Printf("[SoloBotManager] Virtual bot stopped while waiting for opponent in match %s: %v", matchID, ctx.Err())
			return
		case oppProgress, ok := <-oppUpdates:
			if !ok {
				log.Printf("[SoloBotManager] LiveSync closed while waiting in match %s, virtual bot exiting", matchID)
				return
			}
			if oppProgress.Status == "ABORTED" {
				log.Printf("[SoloBotManager] Human opponent aborted while bot waiting in match %s, completing", matchID)
				m.emitBotCompleted(context.Background(), matchID, botUserID, exerciseID, chType, botScore)
				return
			}
			if oppProgress.Status == "ACTIVE" || oppProgress.Status == "COMPLETED" || oppProgress.Status == "WAITING" || oppProgress.Status == "SPECTATING" {
				isActivated = true
			}
		case <-pollTicker.C:
			ready, readyErr := m.liveSync.IsMatchReady(ctx, matchID)
			if readyErr != nil {
				log.Printf("[SoloBotManager] Virtual bot failed to check match readiness for %s: %v", matchID, readyErr)
			} else if ready {
				isActivated = true
			}
		}
	}

	// Giai đoạn 2: Người thật đã vào phòng -> Chuyển sang ACTIVE và bắt đầu tập ngay
	activeProgress := port.ParticipantProgress{
		UserID:    botUserID,
		Value:     0,
		FormScore: 100.0,
		Status:    "ACTIVE",
	}
	if pubErr := m.liveSync.PublishMyProgress(ctx, matchID, activeProgress); pubErr != nil {
		log.Printf("[SoloBotManager] Virtual bot failed to publish ACTIVE status: %v", pubErr)
	}

	// Giai đoạn 3: Bắt đầu thi đấu và đếm số realtime theo nhịp độ
	interval := repInterval
	if chType == vo.ChallengeTypeDuration {
		interval = durInterval
	}

	stepTicker := time.NewTicker(interval)
	defer stepTicker.Stop()

	var currentVal int32
	var earlyTerminated bool

	for currentVal < botScore {
		select {
		case <-ctx.Done():
			log.Printf("[SoloBotManager] Virtual bot stopped streaming due to context cancellation in match %s: %v", matchID, ctx.Err())
			return
		case oppProgress, ok := <-oppUpdates:
			if !ok {
				log.Printf("[SoloBotManager] LiveSync channel closed for match %s, virtual bot completing early", matchID)
				earlyTerminated = true
				break
			}
			if oppProgress.Status == "ABORTED" {
				log.Printf("[SoloBotManager] Human opponent aborted in match %s, virtual bot completing early", matchID)
				earlyTerminated = true
				break
			}
			if oppProgress.Status == "DISCONNECTED" {
				log.Printf("[SoloBotManager] Human opponent disconnected in match %s (grace period active), virtual bot continues exercising", matchID)
			}
		case <-stepTicker.C:
			currentVal++
			status := "ACTIVE"
			if currentVal >= botScore {
				status = "COMPLETED"
			}
			progress := port.ParticipantProgress{
				UserID:    botUserID,
				Value:     currentVal,
				FormScore: 100.0,
				Status:    status,
			}
			if pubErr := m.liveSync.PublishMyProgress(ctx, matchID, progress); pubErr != nil {
				log.Printf("[SoloBotManager] Virtual bot failed to publish progress step %d: %v", currentVal, pubErr)
			}
		}

		if earlyTerminated {
			break
		}
	}

	// Giai đoạn 4: Hoàn thành bài tập & phát sự kiện SoloTrackingCompleted vào Outbox
	completeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	finalProgress := port.ParticipantProgress{
		UserID:    botUserID,
		Value:     botScore,
		FormScore: 100.0,
		Status:    "COMPLETED",
	}
	if pubErr := m.liveSync.PublishMyProgress(completeCtx, matchID, finalProgress); pubErr != nil {
		log.Printf("[SoloBotManager] Virtual bot failed to publish final COMPLETED status for match %s: %v", matchID, pubErr)
	}

	m.emitBotCompleted(completeCtx, matchID, botUserID, exerciseID, chType, botScore)

	select {
	case <-ctx.Done():
		return
	case <-time.After(500 * time.Millisecond):
	}
}

func (m *SoloBotManager) emitBotCompleted(
	ctx context.Context,
	matchID, botUserID, exerciseID string,
	chType vo.ChallengeType,
	score int32,
) {
	if m.outbox == nil {
		return
	}
	durationSec := int32(60)
	if chType == vo.ChallengeTypeDuration {
		durationSec = score
	}

	botEvt := &domainEvent.SoloTrackingCompleted{
		MatchID:              matchID,
		UserID:               botUserID,
		ExerciseID:           exerciseID,
		ChallengeType:        chType,
		ConfirmedValue:       score,
		AverageFormScore:     100.0,
		TotalDurationSeconds: durationSec,
		CompletedAt:          time.Now().UTC(),
	}
	if err := m.outbox.WriteEvents(ctx, "SoloMatch", matchID, []interface{}{botEvt}); err != nil {
		log.Printf("[SoloBotManager] Virtual bot failed to write SoloTrackingCompleted to outbox: %v", err)
	} else {
		log.Printf("[SoloBotManager] Virtual bot successfully emitted SoloTrackingCompleted (score=%d) for match %s", score, matchID)
	}
}

func (m *SoloBotManager) resolveBotScore(ctx context.Context, userID, exerciseID string, chType vo.ChallengeType) int32 {
	if m.prRepo != nil && userID != "" && exerciseID != "" {
		pr, err := m.prRepo.FindByUserIDAndExerciseID(ctx, userID, exerciseID)
		if err != nil {
			log.Printf("[SoloBotManager] Warning: failed to fetch PR for user %s on exercise %s: %v", userID, exerciseID, err)
		} else if pr != nil && pr.Reps() > 0 {
			return int32(pr.Reps())
		}
	}

	if chType == vo.ChallengeTypeDuration {
		return DefaultBotDurationSeconds
	}
	return DefaultBotReps
}

func parseChallengeType(raw string) vo.ChallengeType {
	upper := strings.ToUpper(strings.TrimSpace(raw))
	if upper == "2" || strings.Contains(upper, "DURATION") {
		return vo.ChallengeTypeDuration
	}
	return vo.ChallengeTypeReps
}
