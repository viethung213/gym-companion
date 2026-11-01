package middleware

import (
	// Blank imports to register all compiled protobuf service descriptors into protoregistry.GlobalFiles.
	// This enables the API Gateway to dynamically reflect OpenAPI security annotations as the Single Source of Truth (SSOT).
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/audio/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/service"
	_ "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/service"
)
