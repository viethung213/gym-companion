package webtransport

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/quic-go/quic-go/http3"
	wt "github.com/quic-go/webtransport-go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/rule"
)


type mockLiveSync struct{}

func (m *mockLiveSync) RegisterParticipant(ctx context.Context, matchID, userID, exerciseID string) error {
	return nil
}
func (m *mockLiveSync) UnregisterParticipant(ctx context.Context, matchID, userID string) error {
	return nil
}
func (m *mockLiveSync) IsMatchReady(ctx context.Context, matchID string) (bool, error) {
	return true, nil
}
func (m *mockLiveSync) ParticipantCount(ctx context.Context, matchID string) (int, error) {
	return 2, nil
}
func (m *mockLiveSync) PublishMyProgress(ctx context.Context, matchID string, prog port.ParticipantProgress) error {
	return nil
}
func (m *mockLiveSync) GetMyProgress(ctx context.Context, matchID, userID string) (*port.ParticipantProgress, error) {
	return nil, nil
}
func (m *mockLiveSync) GetOpponentProgress(ctx context.Context, matchID, userID string) (*port.ParticipantProgress, error) {
	return &port.ParticipantProgress{UserID: "opp-1", Status: "COMPLETED", Value: 3, FormScore: 90}, nil
}
func (m *mockLiveSync) SubscribeMatchUpdates(ctx context.Context, matchID, userID string) (<-chan port.ParticipantProgress, func(), error) {
	ch := make(chan port.ParticipantProgress)
	return ch, func() { close(ch) }, nil
}
func (m *mockLiveSync) DelRoom(ctx context.Context, matchID string) error {
	return nil
}

const goodToken = "good-token"

func newTestServer(t *testing.T) *Server {
	t.Helper()
	svc := command.NewSoloTrackingService(rule.NewCachedRuleProvider(nil, nil), service.NewDefaultRuleEvaluator(), &mockLiveSync{}, nil, nil, nil)
	s, err := New(Config{
		Addr:    "127.0.0.1:0",
		Service: svc,
		Validator: func(_ context.Context, token string) (string, string, error) {
			if token == goodToken {
				return "user-wt-1", "User", nil
			}
			return "", "", errors.New("bad token")
		},
		IdleTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// bicepFrame builds a COCO-17 side-view skeleton whose elbow angle (P5-P7-P9) equals elbowDeg.
// Shoulder/hip are fixed, so torso length = 0.30 and spine sway = 0 (a clean rep with no form errors).
func bicepFrame(elbowDeg float64, at time.Time) *msg.StreamSoloMatchTrackingRequest {
	const sx, sy, hx, hy, ex, ey, forearm = 0.50, 0.30, 0.50, 0.60, 0.50, 0.45, 0.14
	rad := elbowDeg * math.Pi / 180
	wx, wy := ex+forearm*math.Sin(rad), ey-forearm*math.Cos(rad)
	pt := func(i int, x, y float64) *msg.SkeletonLandmark {
		return &msg.SkeletonLandmark{LandmarkIndex: int32(i), X: float32(x), Y: float32(y), Visibility: 0.95, Presence: 0.95}
	}
	var lms []*msg.SkeletonLandmark
	for i := 0; i < 17; i++ {
		lms = append(lms, pt(i, 0.5, 0.5))
	}
	for _, i := range []int{5, 6} {
		lms[i] = pt(i, sx, sy)
	}
	for _, i := range []int{7, 8} {
		lms[i] = pt(i, ex, ey)
	}
	for _, i := range []int{9, 10} {
		lms[i] = pt(i, wx, wy)
	}
	for _, i := range []int{11, 12} {
		lms[i] = pt(i, hx, hy)
	}
	return &msg.StreamSoloMatchTrackingRequest{Payload: &msg.StreamSoloMatchTrackingRequest_Frame{
		Frame: &msg.SoloTrackingFrame{Timestamp: timestamppb.New(at), Landmarks: lms, CameraAngle: "side"},
	}}
}

func configReq(matchID string) *msg.StreamSoloMatchTrackingRequest {
	return &msg.StreamSoloMatchTrackingRequest{Payload: &msg.StreamSoloMatchTrackingRequest_Config{
		Config: &msg.SoloTrackingConfig{
			MatchId: matchID, PlanId: "plan-1", ExerciseId: "bicep_curl",
			ChallengeType: msg.SoloChallengeType_SOLO_CHALLENGE_TYPE_REPS, CameraAngle: "side",
		},
	}}
}

// repAngles is one full bicep-curl rep sampled at 20 Hz: 170° -> 60° -> 170°.
func repAngles() []float64 {
	var a []float64
	for i := 0; i <= 12; i++ {
		a = append(a, 170-float64(i)/12*110)
	}
	for i := 0; i < 5; i++ {
		a = append(a, 60)
	}
	for i := 11; i >= 0; i-- {
		a = append(a, 170-float64(i)/12*110)
	}
	return a
}

// exchange abstracts one client transport so the same scenario runs over WebTransport and WebSocket.
type exchange struct {
	send func(*msg.StreamSoloMatchTrackingRequest) error
	recv func() (*msg.StreamSoloMatchTrackingResponse, error)
}

// runBicepScenario streams 3 reps and returns every response received, then submits and returns the final one.
func runBicepScenario(t *testing.T, ex exchange, matchID string) (frames []*msg.StreamSoloMatchTrackingResponse, final *msg.StreamSoloMatchTrackingResponse) {
	t.Helper()
	if err := ex.send(configReq(matchID)); err != nil {
		t.Fatalf("send config: %v", err)
	}
	if _, err := ex.recv(); err != nil {
		t.Fatalf("recv config ack: %v", err)
	}

	ts := time.Now()
	for rep := 0; rep < 3; rep++ {
		for _, ang := range repAngles() {
			ts = ts.Add(50 * time.Millisecond)
			if err := ex.send(bicepFrame(ang, ts)); err != nil {
				t.Fatalf("send frame: %v", err)
			}
			resp, err := ex.recv()
			if err != nil {
				t.Fatalf("recv frame response: %v", err)
			}
			frames = append(frames, resp)
		}
	}

	if err := ex.send(&msg.StreamSoloMatchTrackingRequest{Payload: &msg.StreamSoloMatchTrackingRequest_Submit{Submit: &msg.SoloTrackingSubmit{}}}); err != nil {
		t.Fatalf("send submit: %v", err)
	}
	final, err := ex.recv()
	if err != nil {
		t.Fatalf("recv submit response: %v", err)
	}
	return frames, final
}

func assertBicepOutcome(t *testing.T, frames []*msg.StreamSoloMatchTrackingResponse, final *msg.StreamSoloMatchTrackingResponse) {
	t.Helper()
	last := frames[len(frames)-1]
	if last.MyConfirmedValue != 3 {
		t.Errorf("confirmed reps after 3 real curls = %d, want 3", last.MyConfirmedValue)
	}
	if !last.PersonDetected {
		t.Error("person_detected must be true while a skeleton is streamed")
	}
	var sawActive, sawRest bool
	var minAngle, maxAngle float32 = 999, 0
	for _, f := range frames {
		switch f.RepPhase {
		case "ACTIVE":
			sawActive = true
		case "REST":
			sawRest = true
		}
		if f.CurrentAngle > 0 {
			minAngle = min(minAngle, f.CurrentAngle)
			maxAngle = max(maxAngle, f.CurrentAngle)
		}
		if len(f.DetectedErrors) != 0 {
			t.Fatalf("clean-form curls must emit no confirmed errors, got %v", f.DetectedErrors)
		}
	}
	if !sawActive || !sawRest {
		t.Errorf("server must report rep_phase transitions: sawActive=%v sawRest=%v", sawActive, sawRest)
	}
	// The server computes the angle, not the client: it must track the 60°..170° sweep we streamed.
	if minAngle > 65 || maxAngle < 165 {
		t.Errorf("server-measured angle range [%.1f, %.1f], want ≈[60, 170]", minAngle, maxAngle)
	}
	if !final.MatchEnded || !final.IsCompleted || final.MyConfirmedValue != 3 {
		t.Errorf("submit on single-player set must end the stream with 3 reps, got ended=%v completed=%v reps=%d",
			final.MatchEnded, final.IsCompleted, final.MyConfirmedValue)
	}
}

// pinnedTLS mimics the browser's serverCertificateHashes behaviour: no CA chain, only the SHA-256 pin.
func pinnedTLS(hashB64 string) *tls.Config {
	want, _ := base64.StdEncoding.DecodeString(hashB64)
	return &tls.Config{
		InsecureSkipVerify: true, // chain is replaced by the pin check below
		NextProtos:         []string{http3.NextProtoH3},
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return errors.New("no peer certificate")
			}
			got := sha256.Sum256(raw[0])
			if string(got[:]) != string(want) {
				return errors.New("certificate pin mismatch")
			}
			return nil
		},
	}
}

func dialWT(t *testing.T, s *Server, token string, hashB64 string) (*wt.Session, *http.Response, error) {
	t.Helper()
	port := s.Addr().(interface{ String() string }).String()
	u := fmt.Sprintf("https://%s%s?token=%s", port, WebTransportPath, url.QueryEscape(token))
	tr := &wt.Transport{TLSClientConfig: pinnedTLS(hashB64)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, sess, err := tr.Dial(ctx, u, nil)
	return sess, resp, err
}

func TestWebTransport_RealQUICSession_ServerIsAuthoritativeReferee(t *testing.T) {
	s := newTestServer(t)
	sess, _, err := dialWT(t, s, goodToken, s.CertHashBase64())
	if err != nil {
		t.Fatalf("WebTransport dial over real QUIC failed: %v", err)
	}
	defer sess.CloseWithError(0, "")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stream, err := sess.OpenStreamSync(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	r := bufio.NewReader(stream)
	ex := exchange{
		send: func(req *msg.StreamSoloMatchTrackingRequest) error {
			b, _ := proto.Marshal(req)
			return writeMessage(stream, b)
		},
		recv: func() (*msg.StreamSoloMatchTrackingResponse, error) {
			_ = stream.SetReadDeadline(time.Now().Add(5 * time.Second))
			raw, err := readMessage(r)
			if err != nil {
				return nil, err
			}
			out := &msg.StreamSoloMatchTrackingResponse{}
			return out, proto.Unmarshal(raw, out)
		},
	}
	frames, final := runBicepScenario(t, ex, "live-wt-test-1")
	assertBicepOutcome(t, frames, final)
}

func TestWebTransport_RejectsBadTokenAndWrongPin(t *testing.T) {
	s := newTestServer(t)

	if _, resp, err := dialWT(t, s, "wrong-token", s.CertHashBase64()); err == nil {
		t.Fatal("dial with invalid JWT must fail")
	} else if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid JWT status = %d, want 401", resp.StatusCode)
	}

	wrongPin := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if _, _, err := dialWT(t, s, goodToken, wrongPin); err == nil {
		t.Fatal("dial with a mismatching certificate pin must fail (MITM protection)")
	}
}

func TestConfigHandler_ReturnsPinMatchingServedCertificate(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.ConfigHandler())
	defer ts.Close()

	// No token -> 401
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("config without token = %d, want 401", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "?token=" + goodToken)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var cfg ConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.CertHashB64 != s.CertHashBase64() {
		t.Errorf("config pin %q != server pin %q", cfg.CertHashB64, s.CertHashBase64())
	}
	if !strings.HasPrefix(cfg.URL, "https://127.0.0.1:") || !strings.HasSuffix(cfg.URL, WebTransportPath) {
		t.Errorf("unexpected WebTransport URL %q", cfg.URL)
	}
	exp, err := time.Parse(time.RFC3339, cfg.CertExpiresAt)
	if err != nil || time.Until(exp) > 14*24*time.Hour {
		t.Errorf("cert must expire within 14 days (browser limit for serverCertificateHashes), got %v err=%v", cfg.CertExpiresAt, err)
	}

	// The advertised pin must be usable end-to-end with the URL the config returned.
	u := cfg.URL + "?token=" + goodToken
	tr := &wt.Transport{TLSClientConfig: pinnedTLS(cfg.CertHashB64)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, sess, err := tr.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial using URL+pin from config endpoint: %v", err)
	}
	_ = sess.CloseWithError(0, "")
}

func TestWebSocketFallback_SameProtocolSameReferee(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.WebSocketHandler())
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "?token=" + goodToken

	if _, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"?token=nope", nil); err == nil {
		t.Fatal("websocket with invalid JWT must be rejected")
	} else if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid JWT websocket status = %v, want 401", resp)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	ex := exchange{
		send: func(req *msg.StreamSoloMatchTrackingRequest) error {
			b, _ := proto.Marshal(req)
			return conn.WriteMessage(websocket.BinaryMessage, b)
		},
		recv: func() (*msg.StreamSoloMatchTrackingResponse, error) {
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return nil, err
			}
			out := &msg.StreamSoloMatchTrackingResponse{}
			return out, proto.Unmarshal(raw, out)
		},
	}
	frames, final := runBicepScenario(t, ex, "live-ws-test-1")
	assertBicepOutcome(t, frames, final)
}

func TestFraming_RejectsOversizedAndTruncatedMessages(t *testing.T) {
	var sb strings.Builder
	// uvarint(1<<20) = 0x80 0x80 0x40 -> declared length far above maxMessageBytes
	sb.Write([]byte{0x80, 0x80, 0x40})
	if _, err := readMessage(bufio.NewReader(strings.NewReader(sb.String()))); !errors.Is(err, errMessageTooLarge) {
		t.Errorf("oversized declared length must be rejected, got %v", err)
	}
	if _, err := readMessage(bufio.NewReader(strings.NewReader("\x05ab"))); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("truncated body must be ErrUnexpectedEOF, got %v", err)
	}
}
