package coaching

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

// Handler provides HTTP handlers for coaching operations.
type Handler struct {
	coachAgent port.CoachAgent
}

// GenerateRoadmapRequest is the request body for roadmap generation.
type GenerateRoadmapRequest struct {
	UserID string `json:"user_id"`
}

// GenerateRoadmapResponse is the response body for roadmap generation.
type GenerateRoadmapResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewHandler creates a new coaching HTTP handler.
func NewHandler(coachAgent port.CoachAgent) *Handler {
	return &Handler{
		coachAgent: coachAgent,
	}
}

// NewCoachingHandler is an alias for NewHandler for backwards compatibility.
func NewCoachingHandler(coachAgent port.CoachAgent) *Handler {
	return NewHandler(coachAgent)
}

// HandleGenerateRoadmap handles POST /coaching/roadmap/generate
func (h *Handler) HandleGenerateRoadmap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(GenerateRoadmapResponse{
			Status:  "error",
			Message: "Method not allowed",
			Error:   "Use POST",
		})
		return
	}

	var req GenerateRoadmapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(GenerateRoadmapResponse{
			Status:  "error",
			Message: "Invalid request body",
			Error:   err.Error(),
		})
		return
	}

	if req.UserID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(GenerateRoadmapResponse{
			Status:  "error",
			Message: "user_id is required",
		})
		return
	}

	log.Printf("Generating roadmap for user: %s\n", req.UserID)

	ctx := context.Background()
	roadmap, err := h.coachAgent.GenerateRoadmap(ctx, req.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(GenerateRoadmapResponse{
			Status:  "error",
			Message: "Failed to generate roadmap",
			Error:   err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(GenerateRoadmapResponse{
		Status:  "success",
		Message: "Roadmap generated successfully",
		Data:    roadmap,
	})
}

// RegisterHandlers registers coaching HTTP handlers.
func RegisterHandlers(mux *http.ServeMux, handler *Handler) {
	mux.HandleFunc("/coaching/roadmap/generate", handler.HandleGenerateRoadmap)
	log.Println("Registered coaching HTTP handlers")
}
