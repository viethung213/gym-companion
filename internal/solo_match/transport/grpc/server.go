package grpc

import (
	solomatchv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/service"
	"google.golang.org/grpc"
)

func RegisterSoloMatchServer(s *grpc.Server, handler solomatchv1service.SoloMatchServiceServer) {
	solomatchv1service.RegisterSoloMatchServiceServer(s, handler)
}
