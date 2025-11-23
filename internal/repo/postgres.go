package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"pr-review-service/internal/model"
)

/*
PostgresRepo реализует слой доступа к данным (Repository)
для работы с PostgreSQL.

Он инкапсулирует выполнение SQL-запросов и предоставляет
удобный интерфейс для сервисного слоя, скрывая детали SQL.
*/
type PostgresRepo struct {
	db *sql.DB
}

func NewPostgresRepo(db *sql.DB) *PostgresRepo {
	return &PostgresRepo{db: db}
}

/*
CreateTeamWithMembers создаёт команду и всех её участников
в рамках одной транзакции.
*/
func (r *PostgresRepo) CreateTeamWithMembers(ctx context.Context, t model.Team) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	var exists bool
	err = tx.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM teams WHERE name=$1)", t.TeamName,
	).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return errors.New("team_exists")
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO teams(name) VALUES ($1)", t.TeamName)
	if err != nil {
		return err
	}

	for _, m := range t.Members {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO users(user_id, username, team_name, is_active)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id) DO UPDATE
				SET username = EXCLUDED.username,
					team_name = EXCLUDED.team_name,
					is_active = EXCLUDED.is_active
		`, m.UserID, m.Username, t.TeamName, m.IsActive)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

/*
GetTeam возвращает команду и всех её участников.
*/
func (r *PostgresRepo) GetTeam(ctx context.Context, name string) (*model.Team, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.name, u.user_id, u.username, u.is_active
		FROM teams t
		LEFT JOIN users u ON u.team_name = t.name
		WHERE t.name=$1
	`, name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var team model.Team
	members := []model.TeamMember{}
	found := false

	for rows.Next() {
		found = true
		var tn, uid, uname sql.NullString
		var act sql.NullBool

		if err := rows.Scan(&tn, &uid, &uname, &act); err != nil {
			return nil, err
		}

		team.TeamName = tn.String

		if uid.Valid {
			members = append(members, model.TeamMember{
				UserID:   uid.String,
				Username: uname.String,
				IsActive: act.Bool,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if !found {
		return nil, sql.ErrNoRows
	}

	team.Members = members
	return &team, nil
}

/*
GetUserByID возвращает пользователя по идентификатору.
*/
func (r *PostgresRepo) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT user_id, username, team_name, is_active
		FROM users
		WHERE user_id=$1
	`, id)

	var u model.User
	if err := row.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		return nil, err
	}
	return &u, nil
}

/*
UpdateUserIsActive обновляет флаг активности пользователя.
*/
func (r *PostgresRepo) UpdateUserIsActive(ctx context.Context, id string, active bool) (*model.User, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE users SET is_active=$1 WHERE user_id=$2
		RETURNING user_id, username, team_name, is_active
	`, active, id)

	var u model.User
	if err := row.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		return nil, err
	}
	return &u, nil
}

/*
PRExists проверяет, существует ли Pull Request с указанным ID.
Используется сервисом для обработки ошибки PR_EXISTS.
*/
func (r *PostgresRepo) PRExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM pull_requests WHERE pull_request_id=$1)", id,
	).Scan(&exists)
	return exists, err
}

/*
CreatePullRequest создаёт новый PR и всех его ревьюверов.
*/
func (r *PostgresRepo) CreatePullRequest(ctx context.Context, pr model.PullRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO pull_requests(pull_request_id, pull_request_name, author_id, status, created_at)
		VALUES ($1, $2, $3, 'OPEN', $4)
	`, pr.ID, pr.Name, pr.AuthorID, pr.CreatedAt)
	if err != nil {
		return err
	}

	for _, rID := range pr.AssignedReviewers {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO pull_request_reviewers(pull_request_id, user_id)
			VALUES ($1, $2)
		`, pr.ID, rID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

/*
GetPullRequestWithReviewers возвращает полный объект PR
вместе со списком его ревьюверов.
*/
func (r *PostgresRepo) GetPullRequestWithReviewers(ctx context.Context, id string) (*model.PullRequest, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
		FROM pull_requests
		WHERE pull_request_id=$1
	`, id)

	var pr model.PullRequest
	if err := row.Scan(&pr.ID, &pr.Name, &pr.AuthorID, &pr.Status, &pr.CreatedAt, &pr.MergedAt); err != nil {
		return nil, err
	}

	revRows, err := r.db.QueryContext(ctx, `
		SELECT user_id FROM pull_request_reviewers
		WHERE pull_request_id=$1
	`, pr.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = revRows.Close() }()

	var revs []string
	for revRows.Next() {
		var uid string
		if err := revRows.Scan(&uid); err != nil {
			return nil, err
		}
		revs = append(revs, uid)
	}

	if err := revRows.Err(); err != nil {
		return nil, err
	}

	pr.AssignedReviewers = revs
	return &pr, nil
}

/*
SetPRMerged изменяет статус PR на MERGED и устанавливает merged_at.
*/
func (r *PostgresRepo) SetPRMerged(ctx context.Context, id string, mergedAt sql.NullTime) (*model.PullRequest, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pull_requests
		SET status='MERGED', merged_at=$2
		WHERE pull_request_id=$1
	`, id, mergedAt)
	if err != nil {
		return nil, err
	}

	return r.GetPullRequestWithReviewers(ctx, id)
}

/*
SetPRReviewers заменяет список ревьюверов PR на новый.
*/
func (r *PostgresRepo) SetPRReviewers(ctx context.Context, id string, reviewers []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`DELETE FROM pull_request_reviewers WHERE pull_request_id=$1`,
		id,
	)
	if err != nil {
		return err
	}

	for _, rid := range reviewers {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO pull_request_reviewers(pull_request_id, user_id)
			 VALUES ($1, $2)`, id, rid)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

/*
GetRandomActiveReviewersFromTeamExcluding выбирает случайных активных участников
команды, исключая указанных пользователей.
*/
func (r *PostgresRepo) GetRandomActiveReviewersFromTeamExcluding(
	ctx context.Context, team string, limit int, exclude []string) ([]string, error) {

	ex := "("
	for i, e := range exclude {
		if i == 0 {
			ex += fmt.Sprintf("'%s'", e)
		} else {
			ex += fmt.Sprintf(", '%s'", e)
		}
	}
	ex += ")"

	query := `
		SELECT user_id FROM users
		WHERE team_name=$1 AND is_active=true AND user_id NOT IN ` + ex + `
		ORDER BY random()
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, team, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := []string{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		result = append(result, uid)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

/*
GetPullRequestsByReviewer возвращает список PR,
где пользователь является ревьювером.
*/
func (r *PostgresRepo) GetPullRequestsByReviewer(ctx context.Context, uid string) ([]model.PullRequestShort, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
		FROM pull_requests pr
		JOIN pull_request_reviewers r ON r.pull_request_id = pr.pull_request_id
		WHERE r.user_id=$1
	`, uid)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := []model.PullRequestShort{}
	for rows.Next() {
		var p model.PullRequestShort
		if err := rows.Scan(&p.ID, &p.Name, &p.AuthorID, &p.Status); err != nil {
			return nil, err
		}
		result = append(result, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetReviewerAssignmentStats возвращает количество назначений ревьюверов по каждому пользователю.
func (r *PostgresRepo) GetReviewerAssignmentStats(ctx context.Context) ([]model.ReviewerStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id, COUNT(*) AS assignments
		FROM pull_request_reviewers
		GROUP BY user_id
		ORDER BY assignments DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var stats []model.ReviewerStat
	for rows.Next() {
		var s model.ReviewerStat
		if err := rows.Scan(&s.UserID, &s.Assignments); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}
