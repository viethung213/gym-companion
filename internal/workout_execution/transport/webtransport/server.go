// Package webtransport exposes the authoritative AI-camera referee loop to browsers over WebTransport
// (HTTP/3 over QUIC/UDP), with an automatic WebSocket fallback (TCP) for networks that block UDP.
//
// Both transports carry the same protobuf messages (StreamSoloMatchTrackingRequest/Response) and feed the
// exact same grpc.RunSoloMatchTrackingLoop used by gRPC/ConnectRPC, so there is one referee implementation.
package webtransport

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	wt "github.com/quic-go/webtransport-go"
	"google.golang.org/protobuf/proto"

	workoutexecutionv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	workoutgrpc "github.com/viethung213/gym-companion/internal/workout_execution/transport/grpc"
)

const (
	// WebTransportPath is the HTTP/3 CONNECT path for WebTransport sessions.
	WebTransportPath = "/api/v1/execution/wt"
	// ConfigPath returns the WebTransport URL and certificate pin (served on the main HTTP server).
	ConfigPath = "/api/v1/execution/wt-config"
	// WebSocketPath is the TCP fallback endpoint (served on the main HTTP server).
	WebSocketPath = "/api/v1/execution/wt-stream"

	defaultIdleTimeout   = 30 * time.Second
	acceptStreamTimeout  = 10 * time.Second
	closeFlushGrace      = 500 * time.Millisecond
	certRenewCheckPeriod = time.Hour
)

// Config configures the Server.
type Config struct {
	// Addr is the UDP listen address for HTTP/3, e.g. ":8443". Use ":0" in tests.
	Addr string
	// PublicHost overrides the host placed in the URL returned by the config endpoint.
	// When empty, the host of the incoming config request is used.
	PublicHost string
	// Validator authenticates the JWT carried in the `token` query parameter (browsers cannot set
	// headers on a WebTransport/WebSocket handshake) or an `Authorization: Bearer` header.
	Validator middleware.TokenValidator
	// Service is the referee/persistence service driving each stream.
	Service *command.SoloTrackingService
	// WorkoutService is the pure referee service for individual workout sets (clean DDD).
	WorkoutService *command.WorkoutTrackingService
	// AllowedOrigins restricts browser origins. Empty allows any origin (same posture as the API CORS config);
	// authentication still requires a valid JWT.
	AllowedOrigins []string
	// IdleTimeout closes a stream that sends nothing for this long. Defaults to 30s.
	IdleTimeout time.Duration
}

// Server hosts the WebTransport listener and the HTTP handlers for config and the WebSocket fallback.
type Server struct {
	cfg  Config
	cert atomic.Pointer[certBundle]
	wt   *wt.Server
	udp  net.PacketConn

	mu      sync.Mutex
	started bool
	stopCh  chan struct{}
}

// New builds a Server and generates its ephemeral certificate.
func New(cfg Config) (*Server, error) {
	if cfg.Validator == nil {
		return nil, errors.New("webtransport: token validator is required")
	}
	if cfg.Service == nil && cfg.WorkoutService == nil {
		return nil, errors.New("webtransport: at least one tracking service is required")
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = defaultIdleTimeout
	}
	s := &Server{cfg: cfg, stopCh: make(chan struct{})}

	var hosts []string
	if cfg.PublicHost != "" {
		hosts = append(hosts, cfg.PublicHost)
	}
	bundle, err := newCertBundle(time.Now(), hosts)
	if err != nil {
		return nil, err
	}
	s.cert.Store(bundle)
	return s, nil
}

// Start binds the UDP socket and serves HTTP/3 in the background. It returns once the socket is bound.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("webtransport: already started")
	}

	udpAddr, err := net.ResolveUDPAddr("udp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("resolve udp addr %q: %w", s.cfg.Addr, err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen udp %q: %w", s.cfg.Addr, err)
	}
	s.udp = conn

	mux := http.NewServeMux()
	h3 := &http3.Server{
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
			NextProtos: []string{http3.NextProtoH3},
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				return &s.cert.Load().tlsCert, nil
			},
		},
		QUICConfig: &quic.Config{
			EnableDatagrams:                  true,
			EnableStreamResetPartialDelivery: true,
			MaxIdleTimeout:                   s.cfg.IdleTimeout,
			KeepAlivePeriod:                  10 * time.Second,
		},
	}
	wt.ConfigureHTTP3Server(h3)
	s.wt = &wt.Server{H3: h3, CheckOrigin: s.checkOrigin}
	mux.HandleFunc(WebTransportPath, s.handleWebTransport)

	go func() {
		if serveErr := s.wt.Serve(conn); serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
			slog.Error("webtransport server stopped", slog.Any("err", serveErr))
		}
	}()
	go s.renewLoop()

	s.started = true
	return nil
}

// Addr returns the bound UDP address (valid after Start).
func (s *Server) Addr() net.Addr {
	if s.udp == nil {
		return nil
	}
	return s.udp.LocalAddr()
}

// CertHashBase64 returns the standard-base64 SHA-256 of the current certificate, as required by
// `new WebTransport(url, { serverCertificateHashes: [{ algorithm: "sha-256", value }] })`.
func (s *Server) CertHashBase64() string { return s.cert.Load().hashB64 }

// Close stops the listener and background renewal.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	s.started = false
	close(s.stopCh)
	err := s.wt.Close()
	_ = s.udp.Close()
	return err
}

func (s *Server) renewLoop() {
	t := time.NewTicker(certRenewCheckPeriod)
	defer t.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case now := <-t.C:
			if cur := s.cert.Load(); cur.needsRenewal(now) {
				var hosts []string
				if s.cfg.PublicHost != "" {
					hosts = append(hosts, s.cfg.PublicHost)
				}
				nb, err := newCertBundle(now, hosts)
				if err != nil {
					slog.Error("webtransport certificate renewal failed", slog.Any("err", err))
					continue
				}
				s.cert.Store(nb)
				slog.Info("webtransport certificate renewed")
			}
		}
	}
}

// ConfigResponse is the JSON returned by the config endpoint.
type ConfigResponse struct {
	URL           string `json:"url"`
	CertHashB64   string `json:"certHashBase64"`
	FallbackPath  string `json:"fallbackPath"`
	CertExpiresAt string `json:"certExpiresAt"`
}

// ConfigHandler serves the WebTransport URL and certificate pin so the browser can connect without a CA-issued cert.
func (s *Server) ConfigHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.udp == nil {
			http.Error(w, "webtransport not started", http.StatusServiceUnavailable)
			return
		}
		if _, _, ok := s.authenticate(r); !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		host := s.cfg.PublicHost
		if host == "" {
			host = r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
		}
		port := s.udp.LocalAddr().(*net.UDPAddr).Port
		u := url.URL{Scheme: "https", Host: net.JoinHostPort(strings.Trim(host, "[]"), fmt.Sprint(port)), Path: WebTransportPath}
		bundle := s.cert.Load()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(ConfigResponse{
			URL:           u.String(),
			CertHashB64:   bundle.hashB64,
			FallbackPath:  WebSocketPath,
			CertExpiresAt: bundle.notAfter.UTC().Format(time.RFC3339),
		})
	})
}

// authenticate validates the JWT from the `token` query parameter or Authorization header.
// Tokens are never logged.
func (s *Server) authenticate(r *http.Request) (userID, role string, ok bool) {
	token := r.URL.Query().Get("token")
	if token == "" {
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		}
	}
	if token == "" {
		return "", "", false
	}
	uid, rl, err := s.cfg.Validator(r.Context(), token)
	if err != nil || uid == "" {
		return "", "", false
	}
	return uid, rl, true
}

func (s *Server) checkOrigin(r *http.Request) bool {
	if len(s.cfg.AllowedOrigins) == 0 {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, o := range s.cfg.AllowedOrigins {
		if strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

func (s *Server) handleWebTransport(w http.ResponseWriter, r *http.Request) {
	uid, role, ok := s.authenticate(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	sess, err := s.wt.Upgrade(w, r)
	if err != nil {
		slog.Warn("webtransport upgrade failed", slog.Any("err", err))
		return
	}

	ctx := middleware.WithActor(sess.Context(), uid, role)
	acceptCtx, cancel := context.WithTimeout(ctx, acceptStreamTimeout)
	stream, err := sess.AcceptStream(acceptCtx)
	cancel()
	if err != nil {
		_ = sess.CloseWithError(1, "no stream opened")
		return
	}

	if s.cfg.WorkoutService != nil {
		workoutAdapter := &protoWorkoutStream{
			ctx:         ctx,
			r:           bufio.NewReader(stream),
			w:           stream,
			idle:        s.cfg.IdleTimeout,
			setReadDead: stream.SetReadDeadline,
		}
		if loopErr := workoutgrpc.RunWorkoutTrackingLoop(ctx, workoutAdapter, s.cfg.WorkoutService); loopErr != nil {
			slog.Warn("webtransport workout tracking loop ended with error", slog.String("user_id", uid), slog.Any("err", loopErr))
		}
	} else if s.cfg.Service != nil {
		adapter := &protoStream{
			ctx:         ctx,
			r:           bufio.NewReader(stream),
			w:           stream,
			idle:        s.cfg.IdleTimeout,
			setReadDead: stream.SetReadDeadline,
		}
		if loopErr := workoutgrpc.RunSoloMatchTrackingLoop(ctx, adapter, s.cfg.Service); loopErr != nil {
			slog.Warn("webtransport tracking loop ended with error", slog.String("user_id", uid), slog.Any("err", loopErr))
		}
	}

	// Gracefully finish the write side so the final response is flushed before the session is torn down.
	_ = stream.Close()
	select {
	case <-sess.Context().Done():
	case <-time.After(closeFlushGrace):
	}
	_ = sess.CloseWithError(0, "")
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// WebSocketHandler serves the TCP fallback. Each binary WebSocket message is one protobuf message.
func (s *Server) WebSocketHandler() http.Handler {
	up := wsUpgrader
	up.CheckOrigin = s.checkOrigin
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, role, ok := s.authenticate(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return // Upgrade already wrote the HTTP error.
		}
		defer conn.Close()
		conn.SetReadLimit(maxMessageBytes)

		ctx, cancel := context.WithCancel(middleware.WithActor(r.Context(), uid, role))
		defer cancel()

		if s.cfg.WorkoutService != nil {
			workoutAdapter := &wsWorkoutStream{ctx: ctx, conn: conn, idle: s.cfg.IdleTimeout}
			if loopErr := workoutgrpc.RunWorkoutTrackingLoop(ctx, workoutAdapter, s.cfg.WorkoutService); loopErr != nil {
				slog.Warn("websocket workout tracking loop ended with error", slog.String("user_id", uid), slog.Any("err", loopErr))
			}
		} else if s.cfg.Service != nil {
			adapter := &wsStream{ctx: ctx, conn: conn, idle: s.cfg.IdleTimeout}
			if loopErr := workoutgrpc.RunSoloMatchTrackingLoop(ctx, adapter, s.cfg.Service); loopErr != nil {
				slog.Warn("websocket tracking loop ended with error", slog.String("user_id", uid), slog.Any("err", loopErr))
			}
		}
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	})
}

// protoStream adapts a WebTransport bidi stream (uvarint-framed protobuf) to grpc.SoloMatchStream.
type protoStream struct {
	ctx         context.Context
	r           *bufio.Reader
	w           interface{ Write([]byte) (int, error) }
	idle        time.Duration
	setReadDead func(time.Time) error
}

func (p *protoStream) Context() context.Context { return p.ctx }

func (p *protoStream) Recv() (*workoutexecutionv1message.StreamSoloMatchTrackingRequest, error) {
	if p.setReadDead != nil {
		_ = p.setReadDead(time.Now().Add(p.idle))
	}
	raw, err := readMessage(p.r)
	if err != nil {
		return nil, err
	}
	req := &workoutexecutionv1message.StreamSoloMatchTrackingRequest{}
	if err := proto.Unmarshal(raw, req); err != nil {
		return nil, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

func (p *protoStream) Send(resp *workoutexecutionv1message.StreamSoloMatchTrackingResponse) error {
	raw, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return writeMessage(p.w, raw)
}

// wsStream adapts a WebSocket connection (one protobuf per binary message) to grpc.SoloMatchStream.
type wsStream struct {
	ctx  context.Context
	conn *websocket.Conn
	idle time.Duration
}

func (w *wsStream) Context() context.Context { return w.ctx }

func (w *wsStream) Recv() (*workoutexecutionv1message.StreamSoloMatchTrackingRequest, error) {
	_ = w.conn.SetReadDeadline(time.Now().Add(w.idle))
	mt, raw, err := w.conn.ReadMessage()
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return nil, io.EOF
		}
		return nil, err
	}
	if mt != websocket.BinaryMessage {
		return nil, errors.New("websocket: binary messages required")
	}
	req := &workoutexecutionv1message.StreamSoloMatchTrackingRequest{}
	if err := proto.Unmarshal(raw, req); err != nil {
		return nil, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

func (w *wsStream) Send(resp *workoutexecutionv1message.StreamSoloMatchTrackingResponse) error {
	raw, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	_ = w.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return w.conn.WriteMessage(websocket.BinaryMessage, raw)
}
// protoWorkoutStream adapts a WebTransport stream to grpc.WorkoutMatchStream.
type protoWorkoutStream struct {
	ctx         context.Context
	r           *bufio.Reader
	w           interface{ Write([]byte) (int, error) }
	idle        time.Duration
	setReadDead func(time.Time) error
}

func (p *protoWorkoutStream) Context() context.Context { return p.ctx }

func (p *protoWorkoutStream) Recv() (*workoutexecutionv1message.StreamWorkoutTrackingRequest, error) {
	if p.setReadDead != nil {
		_ = p.setReadDead(time.Now().Add(p.idle))
	}
	raw, err := readMessage(p.r)
	if err != nil {
		return nil, err
	}
	req := &workoutexecutionv1message.StreamWorkoutTrackingRequest{}
	if err := proto.Unmarshal(raw, req); err != nil {
		return nil, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

func (p *protoWorkoutStream) Send(resp *workoutexecutionv1message.StreamWorkoutTrackingResponse) error {
	raw, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return writeMessage(p.w, raw)
}

// wsWorkoutStream adapts a WebSocket connection to grpc.WorkoutMatchStream.
type wsWorkoutStream struct {
	ctx  context.Context
	conn *websocket.Conn
	idle time.Duration
}

func (w *wsWorkoutStream) Context() context.Context { return w.ctx }

func (w *wsWorkoutStream) Recv() (*workoutexecutionv1message.StreamWorkoutTrackingRequest, error) {
	_ = w.conn.SetReadDeadline(time.Now().Add(w.idle))
	mt, raw, err := w.conn.ReadMessage()
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return nil, io.EOF
		}
		return nil, err
	}
	if mt != websocket.BinaryMessage {
		return nil, errors.New("websocket: binary messages required")
	}
	req := &workoutexecutionv1message.StreamWorkoutTrackingRequest{}
	if err := proto.Unmarshal(raw, req); err != nil {
		return nil, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

func (w *wsWorkoutStream) Send(resp *workoutexecutionv1message.StreamWorkoutTrackingResponse) error {
	raw, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	_ = w.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return w.conn.WriteMessage(websocket.BinaryMessage, raw)
}

