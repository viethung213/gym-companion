// Package profileclient provides clients for Profile bounded context.
package profileclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	profilemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service/profilev1serviceconnect"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/shared/telemetry"
	"golang.org/x/net/http2"
)

var _ repository.ProfileClient = (*ConnectProfileClient)(nil)

// ConnectProfileClient queries 100% real biological metrics from Core Service via Gateway.
type ConnectProfileClient struct {
	client         profilev1serviceconnect.ProfileServiceClient
	internalSecret string
}

// NewConnectProfileClient constructs a ConnectProfileClient communicating through API Gateway.
func NewConnectProfileClient(gatewayURL, internalSecret string, httpClient *http.Client) *ConnectProfileClient {
	if httpClient == nil {
		h2cTransport := &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
			ReadIdleTimeout: 30 * time.Second,
			PingTimeout:     5 * time.Second,
		}
		httpClient = &http.Client{
			Transport: telemetry.WrapHTTPTransport(h2cTransport),
			Timeout:   10 * time.Second,
		}
	}
	transport := connecthttp.NewTransport(httpClient, gatewayURL)
	client := profilev1serviceconnect.NewProfileServiceClient(connect.NewClient(transport))
	return &ConnectProfileClient{
		client:         client,
		internalSecret: internalSecret,
	}
}

// GetBiologicalMetrics fetches 100% real biological metrics from Profile service.
func (c *ConnectProfileClient) GetBiologicalMetrics(
	ctx context.Context,
	userID string,
) (*repository.UserBiologicalMetrics, error) {
	if userID == "" {
		return nil, errors.New("user id cannot be empty")
	}

	tokenStr, err := internaltoken.Generate(userID, "Admin", "ai-nutrition", time.Minute, c.internalSecret)
	if err != nil {
		return nil, fmt.Errorf("generate internal token: %w", err)
	}

	req := &profilemsg.GetProfileRequest{UserId: userID}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set(middleware.HeaderInternalToken, tokenStr)

	res, err := c.client.GetProfile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gateway get profile for nutrition user %s: %w", userID, err)
	}

	data := res
	activity := "MODERATELY_ACTIVE"
	switch strings.ToUpper(data.GetExperienceLevel()) {
	case "BEGINNER":
		activity = "LIGHTLY_ACTIVE"
	case "ADVANCED", "EXPERT":
		activity = "VERY_ACTIVE"
	}

	return &repository.UserBiologicalMetrics{
		WeightKg:      float64(data.GetWeightKg()),
		HeightCm:      float64(data.GetHeightCm()),
		Age:           int(data.GetAge()),
		Gender:        data.GetGender(),
		ActivityLevel: activity,
	}, nil
}
