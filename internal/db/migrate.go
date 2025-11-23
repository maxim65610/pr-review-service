package db

import (
	"database/sql"
	"fmt"
)

/*
Migrate выполняет миграцию схемы базы данных PostgreSQL.

Этот метод вызывается один раз при старте приложения и гарантирует,
что все необходимые таблицы и типы существуют
*/
func Migrate(db *sql.DB) error {
	statements := []string{
		// Таблица команд.
		`CREATE TABLE IF NOT EXISTS teams (
			name TEXT PRIMARY KEY
		);`,

		// Таблица пользователей.
		`CREATE TABLE IF NOT EXISTS users (
			user_id   TEXT PRIMARY KEY,
			username  TEXT NOT NULL,
			team_name TEXT NOT NULL REFERENCES teams(name) ON DELETE RESTRICT,
			is_active BOOLEAN NOT NULL DEFAULT TRUE
		);`,

		// Создание enum-типа pr_status: OPEN | MERGED.
		`DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'pr_status') THEN
				CREATE TYPE pr_status AS ENUM ('OPEN', 'MERGED');
			END IF;
		END$$;`,

		// Таблица Pull Requests.
		`CREATE TABLE IF NOT EXISTS pull_requests (
			pull_request_id   TEXT PRIMARY KEY,
			pull_request_name TEXT NOT NULL,
			author_id         TEXT NOT NULL REFERENCES users(user_id),
			status            pr_status NOT NULL DEFAULT 'OPEN',
			created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
			merged_at         TIMESTAMPTZ
		);`,

		// Таблица связи PR и ревьюверов (many-to-many).
		`CREATE TABLE IF NOT EXISTS pull_request_reviewers (
			pull_request_id TEXT NOT NULL REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
			user_id         TEXT NOT NULL REFERENCES users(user_id),
			PRIMARY KEY (pull_request_id, user_id)
		);`,
	}

	for i, stmt := range statements {
		_, err := db.Exec(stmt)
		if err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}

	return nil
}
