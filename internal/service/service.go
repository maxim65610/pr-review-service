package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"pr-review-service/internal/model"
)

/*
Список ошибок бизнес-логики, которые сервис возвращает
и которые затем мапятся в HTTP коды и OpenAPI error codes.
*/
var (
	ErrTeamExists  = errors.New("team_exists")
	ErrPRExists    = errors.New("pr_exists")
	ErrPRMerged    = errors.New("pr_merged")
	ErrNotAssigned = errors.New("not_assigned")
	ErrNoCandidate = errors.New("no_candidate")
	ErrNotFound    = errors.New("not_found")
)

// Интерфейс репозитория
type Repo interface {
	CreateTeamWithMembers(ctx context.Context, t model.Team) error
	GetTeam(ctx context.Context, name string) (*model.Team, error)

	GetUserByID(ctx context.Context, id string) (*model.User, error)
	UpdateUserIsActive(ctx context.Context, id string, active bool) (*model.User, error)

	PRExists(ctx context.Context, id string) (bool, error)
	CreatePullRequest(ctx context.Context, pr model.PullRequest) error
	GetPullRequestWithReviewers(ctx context.Context, id string) (*model.PullRequest, error)
	SetPRMerged(ctx context.Context, id string, mergedAt sql.NullTime) (*model.PullRequest, error)
	SetPRReviewers(ctx context.Context, id string, reviewers []string) error

	GetRandomActiveReviewersFromTeamExcluding(ctx context.Context, team string, limit int, exclude []string) ([]string, error)
	GetPullRequestsByReviewer(ctx context.Context, uid string) ([]model.PullRequestShort, error)

	GetReviewerAssignmentStats(ctx context.Context) ([]model.ReviewerStat, error)
}

/*
Service инкапсулирует бизнес-логику и использует репозиторий
для доступа к базе данных.
*/
type Service struct {
	repo Repo
}

func NewService(r Repo) *Service {
	return &Service{repo: r}
}

/*
CreateTeam создаёт новую команду вместе с её участниками.

Эндпоинт: POST /team/add
*/
func (s *Service) CreateTeam(ctx context.Context, t model.Team) (*model.Team, error) {
	err := s.repo.CreateTeamWithMembers(ctx, t)
	if err != nil {
		if err.Error() == "team_exists" {
			return nil, ErrTeamExists
		}
		return nil, err
	}
	return &t, nil
}

/*
GetTeam возвращает команду и список её участников.

Эндпоинт: GET /team/get?team_name=...
*/
func (s *Service) GetTeam(ctx context.Context, name string) (*model.Team, error) {
	t, err := s.repo.GetTeam(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

/*
SetIsActive обновляет флаг активности пользователя.

Эндпоинт: POST /users/setIsActive.
*/
func (s *Service) SetUserIsActive(ctx context.Context, uid string, active bool) (*model.User, error) {
	u, err := s.repo.UpdateUserIsActive(ctx, uid, active)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

/*
CreatePullRequest создаёт новый PR и автоматически назначает ревьюверов.

Эндпоинт: POST /pullRequest/create.
*/
func (s *Service) CreatePR(ctx context.Context, id, name, author string) (*model.PullRequest, error) {
	exists, err := s.repo.PRExists(ctx, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPRExists
	}

	user, err := s.repo.GetUserByID(ctx, author)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	exclude := []string{author}
	revs, err := s.repo.GetRandomActiveReviewersFromTeamExcluding(ctx, user.TeamName, 2, exclude)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	pr := model.PullRequest{
		ID:                id,
		Name:              name,
		AuthorID:          author,
		Status:            model.PRStatusOpen,
		AssignedReviewers: revs,
		CreatedAt:         &now,
	}

	err = s.repo.CreatePullRequest(ctx, pr)
	if err != nil {
		return nil, err
	}

	return &pr, nil
}

/*
MergePullRequest переводит PR в статус MERGED.

Эндпоинт: POST /pullRequest/merge.
*/
func (s *Service) MergePR(ctx context.Context, id string) (*model.PullRequest, error) {
	pr, err := s.repo.GetPullRequestWithReviewers(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if pr.Status == model.PRStatusMerged {
		return pr, nil
	}

	now := sql.NullTime{Time: time.Now().UTC(), Valid: true}
	return s.repo.SetPRMerged(ctx, id, now)
}

/*
ReassignReviewer заменяет одного ревьювера случайным активным пользователем
из команды старого ревьювера.

Эндпоинт: POST /pullRequest/reassign.
*/
func (s *Service) ReassignReviewer(ctx context.Context, prID, old string) (*model.PullRequest, string, error) {
	pr, err := s.repo.GetPullRequestWithReviewers(ctx, prID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}

	if pr.Status == model.PRStatusMerged {
		return nil, "", ErrPRMerged
	}

	assigned := false
	for _, r := range pr.AssignedReviewers {
		if r == old {
			assigned = true
		}
	}
	if !assigned {
		return nil, "", ErrNotAssigned
	}

	oldUser, err := s.repo.GetUserByID(ctx, old)
	if err != nil {
		return nil, "", ErrNotFound
	}

	exclude := append([]string{old, pr.AuthorID}, pr.AssignedReviewers...)

	candidates, err := s.repo.GetRandomActiveReviewersFromTeamExcluding(
		ctx,
		oldUser.TeamName,
		1,
		exclude,
	)
	if err != nil {
		return nil, "", err
	}

	if len(candidates) == 0 {
		return nil, "", ErrNoCandidate
	}

	newReviewer := candidates[0]

	for i := range pr.AssignedReviewers {
		if pr.AssignedReviewers[i] == old {
			pr.AssignedReviewers[i] = newReviewer
		}
	}

	err = s.repo.SetPRReviewers(ctx, pr.ID, pr.AssignedReviewers)
	if err != nil {
		return nil, "", err
	}

	return pr, newReviewer, nil
}

/*
GetUserReviews возвращает список PR, где пользователь назначен ревьювером.

Эндпоинт: GET /users/getReview.
*/
func (s *Service) GetReviews(ctx context.Context, uid string) ([]model.PullRequestShort, error) {
	return s.repo.GetPullRequestsByReviewer(ctx, uid)
}

/*
GetReviewerStats получает количество назначений на ревью по каждому пользователю

Эндпоинт: GET /stats/reviewerAssignments
*/
func (s *Service) GetReviewerStats(ctx context.Context) ([]model.ReviewerStat, error) {
	return s.repo.GetReviewerAssignmentStats(ctx)
}
