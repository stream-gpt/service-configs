package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/stream-gpt/service-configs/internal/metrics"
	"github.com/stream-gpt/service-configs/internal/model"
)

type ConfigRepository interface {
	Get(ctx context.Context, key string) (*model.Config, error)
	List(ctx context.Context) ([]*model.Config, error)
	BatchGet(ctx context.Context, keys []string) ([]*model.Config, error)
	Upsert(ctx context.Context, cfg *model.Config) (*model.Config, error)
	Delete(ctx context.Context, key string) error
}

type PostgresConfigRepository struct {
	db *sql.DB
	m  *metrics.Metrics
}

func NewPostgresConfigRepository(db *sql.DB, m ...*metrics.Metrics) *PostgresConfigRepository {
	r := &PostgresConfigRepository{db: db}
	if len(m) > 0 {
		r.m = m[0]
	}
	return r
}

func (r *PostgresConfigRepository) observe(operation string, start time.Time) {
	if r.m != nil {
		r.m.PostgresQueryDuration.WithLabelValues(operation).Observe(time.Since(start).Seconds())
	}
}

func (r *PostgresConfigRepository) Get(ctx context.Context, key string) (*model.Config, error) {
	start := time.Now()
	defer r.observe("get", start)

	cfg := &model.Config{}
	err := r.db.QueryRowContext(ctx,
		`SELECT key, value, description, created_at, updated_at FROM configs WHERE key = $1`, key,
	).Scan(&cfg.Key, &cfg.Value, &cfg.Description, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get config %q: %w", key, err)
	}
	return cfg, nil
}

func (r *PostgresConfigRepository) List(ctx context.Context) ([]*model.Config, error) {
	start := time.Now()
	defer r.observe("list", start)

	rows, err := r.db.QueryContext(ctx,
		`SELECT key, value, description, created_at, updated_at FROM configs ORDER BY key`,
	)
	if err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.Config
	for rows.Next() {
		cfg := &model.Config{}
		if err := rows.Scan(&cfg.Key, &cfg.Value, &cfg.Description, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

func (r *PostgresConfigRepository) BatchGet(ctx context.Context, keys []string) ([]*model.Config, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	start := time.Now()
	defer r.observe("batch_get", start)

	// Build query with numbered placeholders: WHERE key IN ($1, $2, $3, ...)
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, k := range keys {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = k
	}

	query := `SELECT key, value, description, created_at, updated_at FROM configs WHERE key IN (` +
		strings.Join(placeholders, ",") + `)`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch get configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.Config
	for rows.Next() {
		cfg := &model.Config{}
		if err := rows.Scan(&cfg.Key, &cfg.Value, &cfg.Description, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

func (r *PostgresConfigRepository) Upsert(ctx context.Context, cfg *model.Config) (*model.Config, error) {
	start := time.Now()
	defer r.observe("upsert", start)

	result := &model.Config{}
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO configs (key, value, description, created_at, updated_at)
		 VALUES ($1, $2, $3, now(), now())
		 ON CONFLICT (key) DO UPDATE SET
		   value = EXCLUDED.value,
		   description = EXCLUDED.description,
		   updated_at = now()
		 RETURNING key, value, description, created_at, updated_at`,
		cfg.Key, cfg.Value, cfg.Description,
	).Scan(&result.Key, &result.Value, &result.Description, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert config %q: %w", cfg.Key, err)
	}
	return result, nil
}

func (r *PostgresConfigRepository) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer r.observe("delete", start)

	res, err := r.db.ExecContext(ctx, `DELETE FROM configs WHERE key = $1`, key)
	if err != nil {
		return fmt.Errorf("delete config %q: %w", key, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("config %q not found", key)
	}
	return nil
}
