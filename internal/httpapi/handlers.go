package httpapi

import (
	"encoding/json"
	"net/http"

	"pr-review-service/internal/model"
	"pr-review-service/internal/service"

	"github.com/gorilla/mux"
)

/*
Handler реализует слой HTTP API поверх сервисного слоя.
*/
type Handler struct {
	svc *service.Service
}

func NewHandler(s *service.Service) *Handler {
	return &Handler{svc: s}
}

// Router регистрирует все маршруты и возвращает готовый mux.Router
func (h *Handler) Router() http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/team/add", h.handleTeamAdd).Methods("POST")
	r.HandleFunc("/team/get", h.handleTeamGet).Methods("GET")

	r.HandleFunc("/users/setIsActive", h.handleSetIsActive).Methods("POST")
	r.HandleFunc("/users/getReview", h.handleUserReviews).Methods("GET")

	r.HandleFunc("/pullRequest/create", h.handlePRCreate).Methods("POST")
	r.HandleFunc("/pullRequest/merge", h.handlePRMerge).Methods("POST")
	r.HandleFunc("/pullRequest/reassign", h.handlePRReassign).Methods("POST")

	r.HandleFunc("/stats/reviewerAssignments", h.handleReviewerStats).Methods("GET")

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("OK")) // фикс errcheck
	})

	return r
}

/*
ErrorCode — перечисление бизнес-ошибок,
которое соответствует OpenAPI.
*/
type ErrorCode string

const (
	CodeTeamExists  ErrorCode = "TEAM_EXISTS"
	CodePRExists    ErrorCode = "PR_EXISTS"
	CodePRMerged    ErrorCode = "PR_MERGED"
	CodeNotAssigned ErrorCode = "NOT_ASSIGNED"
	CodeNoCandidate ErrorCode = "NO_CANDIDATE"
	CodeNotFound    ErrorCode = "NOT_FOUND"
)

/*
ErrorResponse соответствует формату ошибок OpenAPI.
*/
type ErrorResponse struct {
	Error struct {
		Code    ErrorCode `json:"code"`
		Message string    `json:"message"`
	} `json:"error"`
}

// writeError записывает ошибку в правильном OpenAPI-формате
func writeError(w http.ResponseWriter, status int, code ErrorCode, msg string) {
	w.WriteHeader(status)

	resp := ErrorResponse{}
	resp.Error.Code = code
	resp.Error.Message = msg

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Пишем только в HTTP-лог, но не возвращаем ошибку наружу
		_ = err
	}
}

// handleTeamAdd обрабатывает POST /team/add.
func (h *Handler) handleTeamAdd(w http.ResponseWriter, r *http.Request) {
	var t model.Team
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		w.WriteHeader(400)
		return
	}

	team, err := h.svc.CreateTeam(r.Context(), t)
	if err != nil {
		switch err {
		case service.ErrTeamExists:
			writeError(w, 400, CodeTeamExists, "team already exists")
		default:
			w.WriteHeader(500)
		}
		return
	}

	w.WriteHeader(201)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"team": team}); err != nil {
		_ = err
	}
}

// handleTeamGet обрабатывает GET /team/get?team_name=...
func (h *Handler) handleTeamGet(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("team_name")
	if name == "" {
		w.WriteHeader(400)
		return
	}

	t, err := h.svc.GetTeam(r.Context(), name)
	if err != nil {
		writeError(w, 404, CodeNotFound, "team not found")
		return
	}

	if err := json.NewEncoder(w).Encode(t); err != nil {
		_ = err
	}
}

// handleSetIsActive обрабатывает POST /users/setIsActive.
func (h *Handler) handleSetIsActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		return
	}

	u, err := h.svc.SetUserIsActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		if err == service.ErrNotFound {
			writeError(w, 404, CodeNotFound, "user not found")
			return
		}
		w.WriteHeader(500)
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{"user": u}); err != nil {
		_ = err
	}
}

// handlePRCreate обрабатывает POST /pullRequest/create
func (h *Handler) handlePRCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"pull_request_id"`
		Name   string `json:"pull_request_name"`
		Author string `json:"author_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		return
	}

	pr, err := h.svc.CreatePR(r.Context(), req.ID, req.Name, req.Author)
	if err != nil {
		switch err {
		case service.ErrPRExists:
			writeError(w, 409, CodePRExists, "PR already exists")
		case service.ErrNotFound:
			writeError(w, 404, CodeNotFound, "author not found")
		default:
			w.WriteHeader(500)
		}
		return
	}

	w.WriteHeader(201)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"pr": pr}); err != nil {
		_ = err
	}
}

// handlePRMerge обрабатывает POST /pullRequest/merge
func (h *Handler) handlePRMerge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"pull_request_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		return
	}

	pr, err := h.svc.MergePR(r.Context(), req.ID)
	if err != nil {
		if err == service.ErrNotFound {
			writeError(w, 404, CodeNotFound, "pr not found")
			return
		}
		w.WriteHeader(500)
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{"pr": pr}); err != nil {
		_ = err
	}
}

// handlePRReassign обрабатывает POST /pullRequest/reassign
func (h *Handler) handlePRReassign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID  string `json:"pull_request_id"`
		Old string `json:"old_user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		return
	}

	pr, newReviewer, err := h.svc.ReassignReviewer(r.Context(), req.ID, req.Old)
	if err != nil {
		switch err {
		case service.ErrPRMerged:
			writeError(w, 409, CodePRMerged, "cannot reassign on merged PR")
		case service.ErrNotAssigned:
			writeError(w, 409, CodeNotAssigned, "user not assigned as reviewer")
		case service.ErrNoCandidate:
			writeError(w, 409, CodeNoCandidate, "no candidate available")
		case service.ErrNotFound:
			writeError(w, 404, CodeNotFound, "not found")
		default:
			w.WriteHeader(500)
		}
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"pr":          pr,
		"replaced_by": newReviewer,
	}); err != nil {
		_ = err
	}
}

// handleUserReviews обрабатывает GET /users/getReview?user_id=...
func (h *Handler) handleUserReviews(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("user_id")
	if uid == "" {
		w.WriteHeader(400)
		return
	}

	list, err := h.svc.GetReviews(r.Context(), uid)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":       uid,
		"pull_requests": list,
	}); err != nil {
		_ = err
	}
}

// handleReviewerStats обрабатывает GET /stats/reviewerAssignments
func (h *Handler) handleReviewerStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetReviewerStats(r.Context())
	if err != nil {
		w.WriteHeader(500)
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"stats": stats,
	}); err != nil {
		_ = err
	}
}
