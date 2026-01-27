package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Repo represents a repository.
type Repo struct {
	ID        string
	Name      string
	GitURL    *string
	CreatedAt time.Time
}

// ErrNotFound is returned when an entity is not found.
var ErrNotFound = errors.New("not found")

// CreateRepo creates a new repository.
func (s *Store) CreateRepo(name string, gitURL *string) (*Repo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	now := time.Now().Unix()

	_, err := s.db.Exec(
		"INSERT INTO repos (id, name, git_url, created_at) VALUES (?, ?, ?, ?)",
		id, name, gitURL, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert repo: %w", err)
	}

	return &Repo{
		ID:        id,
		Name:      name,
		GitURL:    gitURL,
		CreatedAt: time.Unix(now, 0),
	}, nil
}

// GetRepo retrieves a repository by ID.
func (s *Store) GetRepo(id string) (*Repo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var repo Repo
	var createdAt int64

	err := s.db.QueryRow(
		"SELECT id, name, git_url, created_at FROM repos WHERE id = ?",
		id,
	).Scan(&repo.ID, &repo.Name, &repo.GitURL, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query repo: %w", err)
	}

	repo.CreatedAt = time.Unix(createdAt, 0)
	return &repo, nil
}

// GetRepoByName retrieves a repository by name.
func (s *Store) GetRepoByName(name string) (*Repo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var repo Repo
	var createdAt int64

	err := s.db.QueryRow(
		"SELECT id, name, git_url, created_at FROM repos WHERE name = ?",
		name,
	).Scan(&repo.ID, &repo.Name, &repo.GitURL, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query repo by name: %w", err)
	}

	repo.CreatedAt = time.Unix(createdAt, 0)
	return &repo, nil
}

// ListRepos retrieves all repositories.
func (s *Store) ListRepos() ([]*Repo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT id, name, git_url, created_at FROM repos ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("query repos: %w", err)
	}
	defer rows.Close()

	var repos []*Repo
	for rows.Next() {
		var repo Repo
		var createdAt int64
		if err := rows.Scan(&repo.ID, &repo.Name, &repo.GitURL, &createdAt); err != nil {
			return nil, fmt.Errorf("scan repo: %w", err)
		}
		repo.CreatedAt = time.Unix(createdAt, 0)
		repos = append(repos, &repo)
	}

	return repos, rows.Err()
}

// UpdateRepo updates a repository's name and git URL.
func (s *Store) UpdateRepo(id, name string, gitURL *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(
		"UPDATE repos SET name = ?, git_url = ? WHERE id = ?",
		name, gitURL, id,
	)
	if err != nil {
		return fmt.Errorf("update repo: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteRepo deletes a repository by ID.
func (s *Store) DeleteRepo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM repos WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete repo: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}
