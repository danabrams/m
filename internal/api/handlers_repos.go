package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/anthropics/m/internal/store"
)

// repoResponse represents a repo in API responses.
type repoResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	GitURL    *string `json:"git_url,omitempty"`
	CreatedAt string  `json:"created_at"`
}

func toRepoResponse(r *store.Repo) repoResponse {
	return repoResponse{
		ID:        r.ID,
		Name:      r.Name,
		GitURL:    r.GitURL,
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}
}

// handleListRepos returns all repositories.
func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := s.store.ListRepos()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list repos")
		return
	}

	resp := make([]repoResponse, len(repos))
	for i, repo := range repos {
		resp[i] = toRepoResponse(repo)
	}

	writeJSON(w, http.StatusOK, resp)
}

// createRepoRequest is the request body for creating a repo.
type createRepoRequest struct {
	Name   string  `json:"name"`
	GitURL *string `json:"git_url,omitempty"`
}

// handleCreateRepo creates a new repository.
func (s *Server) handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	var req createRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "invalid JSON body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "name is required")
		return
	}

	repo, err := s.store.CreateRepo(req.Name, req.GitURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create repo")
		return
	}

	writeJSON(w, http.StatusCreated, toRepoResponse(repo))
}

// handleGetRepo returns a single repository by ID.
func (s *Server) handleGetRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "id is required")
		return
	}

	repo, err := s.store.GetRepo(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "repo not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get repo")
		return
	}

	writeJSON(w, http.StatusOK, toRepoResponse(repo))
}

// handleDeleteRepo deletes a repository by ID.
func (s *Server) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "id is required")
		return
	}

	err := s.store.DeleteRepo(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "repo not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete repo")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
