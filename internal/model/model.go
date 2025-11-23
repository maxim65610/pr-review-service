package model

import "time"

// TeamMember описывает участника команды
type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// Team представляет команду и её участников.
type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

// User представляет пользователя
type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

type PullRequestStatus string

const (
	// PRStatusOpen означает, что Pull Request находится в открытом состоянии
	// и допускает изменение списка ревьюверов
	PRStatusOpen PullRequestStatus = "OPEN"

	// PRStatusMerged означает, что Pull Request был замёрджен.
	// В этом состоянии изменение списка ревьюверов запрещено
	PRStatusMerged PullRequestStatus = "MERGED"
)

// PullRequest описывает сущность PR
type PullRequest struct {
	ID                string            `json:"pull_request_id"`
	Name              string            `json:"pull_request_name"`
	AuthorID          string            `json:"author_id"`
	Status            PullRequestStatus `json:"status"`
	AssignedReviewers []string          `json:"assigned_reviewers"`
	CreatedAt         *time.Time        `json:"createdAt,omitempty"`
	MergedAt          *time.Time        `json:"mergedAt,omitempty"`
}

// PullRequestShort — сокращённое представление PR,
type PullRequestShort struct {
	ID       string            `json:"pull_request_id"`
	Name     string            `json:"pull_request_name"`
	AuthorID string            `json:"author_id"`
	Status   PullRequestStatus `json:"status"`
}

type ReviewerStat struct {
	UserID      string `json:"user_id"`
	Assignments int    `json:"assignments"`
}
