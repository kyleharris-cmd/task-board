package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
)

type Server struct {
	svc *app.Service
}

func NewServer(svc *app.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /tasks", s.handleListTasks)
	mux.HandleFunc("POST /tasks", s.handleCreateTask)
	mux.HandleFunc("POST /tasks/", s.handleTaskAction)
	mux.HandleFunc("DELETE /tasks/", s.handleDeleteTask)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	var state *domain.State
	includeArchived := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_archived")), "true") || strings.TrimSpace(r.URL.Query().Get("include_archived")) == "1"
	if raw := strings.TrimSpace(r.URL.Query().Get("state")); raw != "" {
		parsed, err := domain.ParseState(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		state = &parsed
	}

	tasks, err := s.svc.ListTasksWithArchived(r.Context(), state, includeArchived)
	if err != nil {
		writeError(w, mapStatus(err), err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

type createTaskRequest struct {
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	TaskType          string  `json:"task_type"`
	Priority          int     `json:"priority"`
	ParentID          *string `json:"parent_id"`
	RequiredForParent bool    `json:"required_for_parent"`
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}

	taskID, err := s.svc.CreateTask(r.Context(), app.CreateTaskInput{
		Title:             req.Title,
		Description:       req.Description,
		TaskType:          req.TaskType,
		Priority:          req.Priority,
		ParentID:          req.ParentID,
		RequiredForParent: req.RequiredForParent,
	})
	if err != nil {
		writeError(w, mapStatus(err), err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"task_id": taskID})
}

func (s *Server) handleTaskAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/tasks/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, errors.New("not found"))
		return
	}

	taskID := parts[0]
	action := parts[1]

	switch action {
	case "claim":
		s.handleClaimTask(w, r, taskID)
	case "renew":
		s.handleRenewLease(w, r, taskID)
	case "release":
		s.handleReleaseLease(w, r, taskID)
	case "transition":
		s.handleTransitionTask(w, r, taskID)
	case "artifacts":
		s.handleAddArtifact(w, r, taskID)
	case "rubric":
		s.handleRubric(w, r, taskID)
	case "ready-check":
		s.handleReadyCheck(w, r, taskID)
	case "archive":
		s.handleArchiveTask(w, r, taskID)
	default:
		writeError(w, http.StatusNotFound, errors.New("not found"))
	}
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/tasks/"), "/")
	if taskID == "" || strings.Contains(taskID, "/") {
		writeError(w, http.StatusNotFound, errors.New("not found"))
		return
	}
	force := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("force")), "true") || strings.TrimSpace(r.URL.Query().Get("force")) == "1"
	if err := s.svc.DeleteTask(r.Context(), taskID, force); err != nil {
		writeError(w, mapStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type actorJSON struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func (a actorJSON) parse() (domain.Actor, error) {
	actorType, err := domain.ParseActorType(a.Type)
	if err != nil {
		return domain.Actor{}, err
	}
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.DisplayName) == "" {
		return domain.Actor{}, errors.New("actor.id and actor.display_name are required")
	}
	return domain.Actor{Type: actorType, ID: strings.TrimSpace(a.ID), DisplayName: strings.TrimSpace(a.DisplayName)}, nil
}

type claimRequest struct {
	Actor      actorJSON `json:"actor"`
	TTLMinutes int       `json:"ttl_minutes"`
	AutoRenew  bool      `json:"auto_renew"`
}

func (s *Server) handleClaimTask(w http.ResponseWriter, r *http.Request, taskID string) {
	var req claimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	actor, err := req.Actor.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	until, err := s.svc.ClaimTask(r.Context(), app.ClaimTaskInput{TaskID: taskID, Actor: actor, TTLMinutes: req.TTLMinutes, AutoRenew: req.AutoRenew})
	if err != nil {
		writeError(w, mapStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"lease_expires_at": until.Format("2006-01-02T15:04:05Z07:00")})
}

type renewRequest struct {
	Actor      actorJSON `json:"actor"`
	TTLMinutes int       `json:"ttl_minutes"`
}

func (s *Server) handleRenewLease(w http.ResponseWriter, r *http.Request, taskID string) {
	var req renewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	actor, err := req.Actor.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	until, err := s.svc.RenewTaskLease(r.Context(), taskID, actor, req.TTLMinutes)
	if err != nil {
		writeError(w, mapStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"lease_expires_at": until.Format("2006-01-02T15:04:05Z07:00")})
}

type releaseRequest struct {
	Actor actorJSON `json:"actor"`
}

func (s *Server) handleReleaseLease(w http.ResponseWriter, r *http.Request, taskID string) {
	var req releaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	actor, err := req.Actor.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ReleaseTaskLease(r.Context(), taskID, actor); err != nil {
		writeError(w, mapStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "released"})
}

type transitionRequest struct {
	Actor  actorJSON `json:"actor"`
	To     string    `json:"to"`
	Reason string    `json:"reason"`
}

func (s *Server) handleTransitionTask(w http.ResponseWriter, r *http.Request, taskID string) {
	var req transitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	actor, err := req.Actor.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	toState, err := domain.ParseState(req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.TransitionTask(r.Context(), app.TransitionInput{TaskID: taskID, ToState: toState, Actor: actor, Reason: req.Reason}); err != nil {
		writeError(w, mapStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"state": string(toState)})
}

type artifactRequest struct {
	Actor   actorJSON `json:"actor"`
	Type    string    `json:"type"`
	Content string    `json:"content"`
}

func (s *Server) handleAddArtifact(w http.ResponseWriter, r *http.Request, taskID string) {
	var req artifactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	actor, err := req.Actor.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	artifactType, err := domain.ParseArtifactType(req.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	path, version, err := s.svc.AddArtifact(r.Context(), taskID, artifactType, req.Content, actor)
	if err != nil {
		writeError(w, mapStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "version": version})
}

type rubricRequest struct {
	Actor                  actorJSON `json:"actor"`
	RubricVersion          string    `json:"rubric_version"`
	RequiredFieldsComplete bool      `json:"required_fields_complete"`
	Pass                   bool      `json:"pass"`
	Notes                  string    `json:"notes"`
}

func (s *Server) handleRubric(w http.ResponseWriter, r *http.Request, taskID string) {
	var req rubricRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	actor, err := req.Actor.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.RubricVersion) == "" {
		req.RubricVersion = "v1"
	}
	if err := s.svc.EvaluateRubric(r.Context(), taskID, req.RubricVersion, req.RequiredFieldsComplete, req.Pass, req.Notes, actor); err != nil {
		writeError(w, mapStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

type readyCheckRequest struct {
	Actor actorJSON `json:"actor"`
}

func (s *Server) handleReadyCheck(w http.ResponseWriter, r *http.Request, taskID string) {
	var req readyCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	actor, err := req.Actor.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ReadyCheck(r.Context(), taskID, actor); err != nil {
		writeError(w, mapStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleArchiveTask(w http.ResponseWriter, r *http.Request, taskID string) {
	if err := s.svc.ArchiveTask(r.Context(), taskID); err != nil {
		writeError(w, mapStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

func mapStatus(err error) int {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not found"):
		return http.StatusNotFound
	case strings.Contains(msg, "actively leased"), strings.Contains(msg, "owned by"), strings.Contains(msg, "expired"):
		return http.StatusConflict
	case strings.Contains(msg, "invalid"), strings.Contains(msg, "required"), strings.Contains(msg, "--force"), strings.Contains(msg, "child tasks"), strings.Contains(msg, "already archived"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
