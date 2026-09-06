package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver for database/sql
	"github.com/pressly/goose/v3"

	"github.com/Udassi-Pawan/nimbus/internal/models"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func RunMigrations(databaseURL, migrationsDir string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func (s *Store) ListTeams(ctx context.Context) ([]models.Team, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, name, slug, created_at
		FROM teams
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name, &t.Slug, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (s *Store) ListServices(ctx context.Context) ([]models.Service, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, team_id, name, slug, description, repository_url, owner_email, created_at, updated_at
		FROM services
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var services []models.Service
	for rows.Next() {
		var svc models.Service
		if err := rows.Scan(
			&svc.ID, &svc.TeamID, &svc.Name, &svc.Slug,
			&svc.Description, &svc.RepositoryURL, &svc.OwnerEmail,
			&svc.CreatedAt, &svc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, svc)
	}
	return services, rows.Err()
}

func (s *Store) CreateService(ctx context.Context, input models.CreateServiceInput) (models.Service, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Service{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var svc models.Service
	err = tx.QueryRow(ctx, `
		INSERT INTO services (team_id, name, slug, description, repository_url, owner_email)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, team_id, name, slug, description, repository_url, owner_email, created_at, updated_at
	`, input.TeamID, input.Name, input.Slug, input.Description, input.RepositoryURL, input.OwnerEmail).Scan(
		&svc.ID, &svc.TeamID, &svc.Name, &svc.Slug,
		&svc.Description, &svc.RepositoryURL, &svc.OwnerEmail,
		&svc.CreatedAt, &svc.UpdatedAt,
	)
	if err != nil {
		return models.Service{}, fmt.Errorf("insert service: %w", err)
	}

	envNames := input.Environments
	if len(envNames) == 0 {
		envNames = []string{"dev", "staging", "prod"}
	}

	for _, envName := range envNames {
		namespace := fmt.Sprintf("%s-%s", svc.Slug, envName)
		var env models.ServiceEnvironment
		err = tx.QueryRow(ctx, `
			INSERT INTO service_environments (service_id, name, namespace)
			VALUES ($1, $2, $3)
			RETURNING id, service_id, name, namespace, created_at
		`, svc.ID, envName, namespace).Scan(
			&env.ID, &env.ServiceID, &env.Name, &env.Namespace, &env.CreatedAt,
		)
		if err != nil {
			return models.Service{}, fmt.Errorf("insert environment: %w", err)
		}
		svc.Environments = append(svc.Environments, env)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Service{}, fmt.Errorf("commit tx: %w", err)
	}
	return svc, nil
}