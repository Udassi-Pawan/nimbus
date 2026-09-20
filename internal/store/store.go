package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver for database/sql
	"github.com/pressly/goose/v3"

	"github.com/Udassi-Pawan/nimbus/internal/models"
)

var ErrNotFound = errors.New("not found")
var ErrDuplicateSlug = errors.New("duplicate slug")

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
	if teams == nil {
		teams = []models.Team{}
	}
	return teams, rows.Err()
}

func (s *Store) ListServices(ctx context.Context) ([]models.Service, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT`+serviceSelectColumns+`
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
			&svc.TemplateID, &svc.WorkloadType,
			&svc.CreatedAt, &svc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, svc)
	}
	if services == nil {
		services = []models.Service{}
	}
	return services, rows.Err()
}

func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Store) CreateService(ctx context.Context, input models.CreateServiceInput) (models.Service, error) {
	templateID := input.TemplateID
	if templateID == "" {
		templateID = "go-api"
	}
	workloadType := input.WorkloadType
	if workloadType == "" {
		workloadType = "stateless"
		if templateID == "postgres" || templateID == "redis" {
			workloadType = "stateful"
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Service{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var svc models.Service
	err = tx.QueryRow(ctx, `
		INSERT INTO services (team_id, name, slug, description, repository_url, owner_email, template_id, workload_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING`+serviceSelectColumns+`
	`, input.TeamID, input.Name, input.Slug, input.Description, input.RepositoryURL, input.OwnerEmail, templateID, workloadType).Scan(
		&svc.ID, &svc.TeamID, &svc.Name, &svc.Slug,
		&svc.Description, &svc.RepositoryURL, &svc.OwnerEmail,
		&svc.TemplateID, &svc.WorkloadType,
		&svc.CreatedAt, &svc.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return models.Service{}, ErrDuplicateSlug
		}
		return models.Service{}, fmt.Errorf("insert service: %w", err)
	}

	envNames := input.Environments
	if len(envNames) == 0 {
		envNames = []string{"dev", "staging", "prod"}
	}

	for _, envName := range envNames {
		namespace := fmt.Sprintf("%s-%s", svc.Slug, envName)
		var env models.ServiceEnvironment
		err = scanServiceEnvironment(
			tx.QueryRow(ctx, `
				INSERT INTO service_environments (service_id, name, namespace)
				VALUES ($1, $2, $3)
				RETURNING`+envReturningColumns+`
			`, svc.ID, envName, namespace),
			&env,
		)
		if err != nil {
			if isDuplicateKey(err) {
				return models.Service{}, ErrDuplicateSlug
			}
			return models.Service{}, fmt.Errorf("insert environment: %w", err)
		}
		svc.Environments = append(svc.Environments, env)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Service{}, fmt.Errorf("commit tx: %w", err)
	}
	return svc, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (models.User, string, error) {
	var user models.User
	var passwordHash string

	err := s.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, created_at
		FROM users WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &user.Name, &passwordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, "", ErrNotFound
		}
		return models.User{}, "", fmt.Errorf("get user by email: %w", err)
	}
	return user, passwordHash, nil
}

func (s *Store) CreateAuditLog(ctx context.Context, actorUserID *string, action, resourceType, resourceID string, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, resource_type, resource_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, actorUserID, action, resourceType, resourceID, payload)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func (s *Store) GetService(ctx context.Context, id string) (models.Service, error) {
	var svc models.Service
	err := s.pool.QueryRow(ctx, `
		SELECT`+serviceSelectColumns+`
		FROM services
		WHERE id = $1
	`, id).Scan(
		&svc.ID, &svc.TeamID, &svc.Name, &svc.Slug,
		&svc.Description, &svc.RepositoryURL, &svc.OwnerEmail,
		&svc.TemplateID, &svc.WorkloadType,
		&svc.CreatedAt, &svc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Service{}, ErrNotFound
		}
		return models.Service{}, fmt.Errorf("get service: %w", err)
	}

	envs, err := s.listServiceEnvironments(ctx, svc.ID)
	if err != nil {
		return models.Service{}, err
	}
	svc.Environments = envs

	deps, err := s.listServiceDependencies(ctx, svc.ID)
	if err != nil {
		return models.Service{}, err
	}
	svc.Dependencies = deps
	return svc, nil
}

func (s *Store) listServiceEnvironments(ctx context.Context, serviceID string) ([]models.ServiceEnvironment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT`+envSelectColumns+`
		FROM service_environments
		WHERE service_id = $1
		ORDER BY name
	`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list service environments: %w", err)
	}
	defer rows.Close()

	var envs []models.ServiceEnvironment
	for rows.Next() {
		var env models.ServiceEnvironment
		if err := scanServiceEnvironment(rows, &env); err != nil {
			return nil, fmt.Errorf("scan environment: %w", err)
		}
		envs = append(envs, env)
	}
	return envs, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanServiceEnvironment(row rowScanner, env *models.ServiceEnvironment) error {
	return row.Scan(
		&env.ID, &env.ServiceID, &env.Name, &env.Namespace,
		&env.DeploymentStatus, &env.DeploymentImage,
		&env.DeploymentReplicasDesired, &env.DeploymentReplicasReady,
		&env.StorageSize, &env.StorageClass, &env.SecretName, &env.PVCPhase,
		&env.LastDeployedAt, &env.CreatedAt,
	)
}

func (s *Store) GetServiceEnvironmentByName(ctx context.Context, serviceID, envName string) (models.ServiceEnvironment, error) {
	var env models.ServiceEnvironment
	err := scanServiceEnvironment(s.pool.QueryRow(ctx, `
		SELECT`+envSelectColumns+`
		FROM service_environments
		WHERE service_id = $1 AND name = $2
	`, serviceID, envName), &env)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ServiceEnvironment{}, ErrNotFound
		}
		return models.ServiceEnvironment{}, fmt.Errorf("get service environment: %w", err)
	}
	return env, nil
}

func (s *Store) UpdateEnvironmentDeployment(
	ctx context.Context,
	envID string,
	status string,
	image string,
	replicasDesired, replicasReady int,
	storageSize, storageClass, secretName, pvcPhase string,
) (models.ServiceEnvironment, error) {
	var env models.ServiceEnvironment
	err := scanServiceEnvironment(s.pool.QueryRow(ctx, `
		UPDATE service_environments
		SET deployment_status = $2,
		    deployment_image = $3,
		    deployment_replicas_desired = $4,
		    deployment_replicas_ready = $5,
		    storage_size = COALESCE(NULLIF($6, ''), storage_size),
		    storage_class = COALESCE(NULLIF($7, ''), storage_class),
		    secret_name = COALESCE(NULLIF($8, ''), secret_name),
		    pvc_phase = COALESCE(NULLIF($9, ''), pvc_phase),
		    last_deployed_at = NOW()
		WHERE id = $1
		RETURNING`+envReturningColumns+`
	`, envID, status, image, replicasDesired, replicasReady, storageSize, storageClass, secretName, pvcPhase), &env)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ServiceEnvironment{}, ErrNotFound
		}
		return models.ServiceEnvironment{}, fmt.Errorf("update environment deployment: %w", err)
	}
	return env, nil
}

func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_user_id, action, resource_type, resource_id, metadata, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		var metadata []byte
		var actorUserID *string
		if err := rows.Scan(
			&log.ID, &actorUserID, &log.Action, &log.ResourceType, &log.ResourceID, &metadata, &log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		log.ActorUserID = actorUserID
		if err := json.Unmarshal(metadata, &log.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
		logs = append(logs, log)
	}
	if logs == nil {
		logs = []models.AuditLog{}
	}
	return logs, rows.Err()
}

func (s *Store) listServiceDependencies(ctx context.Context, serviceID string) ([]models.ServiceDependency, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id, d.service_id, d.depends_on_service_id, d.created_at,
		       s.name, s.slug, s.template_id
		FROM service_dependencies d
		JOIN services s ON s.id = d.depends_on_service_id
		WHERE d.service_id = $1
		ORDER BY s.name
	`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list dependencies: %w", err)
	}
	defer rows.Close()

	var deps []models.ServiceDependency
	for rows.Next() {
		var d models.ServiceDependency
		if err := rows.Scan(
			&d.ID, &d.ServiceID, &d.DependsOnServiceID, &d.CreatedAt,
			&d.DependsOnName, &d.DependsOnSlug, &d.DependsOnTemplate,
		); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, d)
	}
	if deps == nil {
		deps = []models.ServiceDependency{}
	}
	return deps, rows.Err()
}

func (s *Store) ListServiceDependencies(ctx context.Context, serviceID string) ([]models.ServiceDependency, error) {
	var exists string
	if err := s.pool.QueryRow(ctx, `SELECT id FROM services WHERE id = $1`, serviceID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lookup service: %w", err)
	}
	return s.listServiceDependencies(ctx, serviceID)
}

func (s *Store) AddServiceDependency(ctx context.Context, serviceID, dependsOnID string) (models.ServiceDependency, error) {
	if serviceID == dependsOnID {
		return models.ServiceDependency{}, fmt.Errorf("service cannot depend on itself")
	}
	var exists string
	if err := s.pool.QueryRow(ctx, `SELECT id FROM services WHERE id = $1`, serviceID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ServiceDependency{}, ErrNotFound
		}
		return models.ServiceDependency{}, fmt.Errorf("lookup service: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT id FROM services WHERE id = $1`, dependsOnID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ServiceDependency{}, ErrNotFound
		}
		return models.ServiceDependency{}, fmt.Errorf("lookup dependency target: %w", err)
	}

	var d models.ServiceDependency
	err := s.pool.QueryRow(ctx, `
		INSERT INTO service_dependencies (service_id, depends_on_service_id)
		VALUES ($1, $2)
		RETURNING id, service_id, depends_on_service_id, created_at
	`, serviceID, dependsOnID).Scan(&d.ID, &d.ServiceID, &d.DependsOnServiceID, &d.CreatedAt)
	if err != nil {
		if isDuplicateKey(err) {
			return models.ServiceDependency{}, ErrDuplicateSlug
		}
		return models.ServiceDependency{}, fmt.Errorf("insert dependency: %w", err)
	}

	deps, err := s.listServiceDependencies(ctx, serviceID)
	if err != nil {
		return models.ServiceDependency{}, err
	}
	for _, dep := range deps {
		if dep.ID == d.ID {
			return dep, nil
		}
	}
	return d, nil
}

func (s *Store) DeleteServiceDependency(ctx context.Context, serviceID, dependencyID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM service_dependencies
		WHERE id = $1 AND service_id = $2
	`, dependencyID, serviceID)
	if err != nil {
		return fmt.Errorf("delete dependency: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}