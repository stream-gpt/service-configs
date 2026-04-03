package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/Gen-Do/service-configs/internal/metrics"
	"github.com/Gen-Do/service-configs/internal/model"
	"github.com/Gen-Do/service-configs/internal/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer     = otel.Tracer("service-configs")
	keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,254}$`)
)

type ConfigService struct {
	repo repository.ConfigRepository
	m    *metrics.Metrics
}

func NewConfigService(repo repository.ConfigRepository, m ...*metrics.Metrics) *ConfigService {
	svc := &ConfigService{repo: repo}
	if len(m) > 0 {
		svc.m = m[0]
	}
	return svc
}

func (s *ConfigService) Get(ctx context.Context, key string) (*model.Config, error) {
	ctx, span := tracer.Start(ctx, "configs.get", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("configs.key", key))

	cfg, err := s.repo.Get(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		s.recordOp("get", "failed")
		return nil, fmt.Errorf("get config: %w", err)
	}

	if cfg == nil {
		s.recordOp("get", "not_found")
		return nil, nil
	}

	s.recordOp("get", "success")
	return cfg, nil
}

func (s *ConfigService) List(ctx context.Context) ([]*model.Config, error) {
	ctx, span := tracer.Start(ctx, "configs.list", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	configs, err := s.repo.List(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		s.recordOp("list", "failed")
		return nil, fmt.Errorf("list configs: %w", err)
	}

	s.recordOp("list", "success")
	return configs, nil
}

func (s *ConfigService) BatchGet(ctx context.Context, keys []string) (map[string]*model.Config, error) {
	ctx, span := tracer.Start(ctx, "configs.batch_get", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.Int("configs.keys_count", len(keys)))

	configs, err := s.repo.BatchGet(ctx, keys)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		s.recordOp("batch_get", "failed")
		return nil, fmt.Errorf("batch get configs: %w", err)
	}

	result := make(map[string]*model.Config, len(configs))
	for _, cfg := range configs {
		result[cfg.Key] = cfg
	}

	s.recordOp("batch_get", "success")
	return result, nil
}

func (s *ConfigService) Create(ctx context.Context, key string, value json.RawMessage, description string) (*model.Config, error) {
	ctx, span := tracer.Start(ctx, "configs.create", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("configs.key", key))

	if err := validateKey(key); err != nil {
		span.RecordError(err)
		s.recordOp("create", "validation_error")
		return nil, err
	}
	if !json.Valid(value) {
		err := fmt.Errorf("invalid JSON value for key %q", key)
		span.RecordError(err)
		s.recordOp("create", "validation_error")
		return nil, err
	}

	// Проверяем, что ключ ещё не существует
	existing, err := s.repo.Get(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		s.recordOp("create", "failed")
		return nil, fmt.Errorf("check existing: %w", err)
	}
	if existing != nil {
		err := fmt.Errorf("config %q already exists", key)
		span.RecordError(err)
		s.recordOp("create", "conflict")
		return nil, err
	}

	cfg := &model.Config{
		Key:         key,
		Value:       value,
		Description: description,
	}

	result, err := s.repo.Upsert(ctx, cfg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		s.recordOp("create", "failed")
		return nil, fmt.Errorf("create config: %w", err)
	}

	s.recordOp("create", "success")
	return result, nil
}

func (s *ConfigService) Update(ctx context.Context, key string, value json.RawMessage, description string) (*model.Config, error) {
	ctx, span := tracer.Start(ctx, "configs.update", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("configs.key", key))

	if !json.Valid(value) {
		err := fmt.Errorf("invalid JSON value for key %q", key)
		span.RecordError(err)
		s.recordOp("update", "validation_error")
		return nil, err
	}

	cfg := &model.Config{
		Key:         key,
		Value:       value,
		Description: description,
	}

	result, err := s.repo.Upsert(ctx, cfg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		s.recordOp("update", "failed")
		return nil, fmt.Errorf("update config: %w", err)
	}

	s.recordOp("update", "success")
	return result, nil
}

func (s *ConfigService) Delete(ctx context.Context, key string) error {
	ctx, span := tracer.Start(ctx, "configs.delete", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("configs.key", key))

	if err := s.repo.Delete(ctx, key); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		s.recordOp("delete", "failed")
		return fmt.Errorf("delete config: %w", err)
	}

	s.recordOp("delete", "success")
	return nil
}

func (s *ConfigService) recordOp(operation, status string) {
	if s.m != nil {
		s.m.OperationsTotal.WithLabelValues(operation, status).Inc()
	}
}

func validateKey(key string) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("invalid config key %q: must match %s", key, keyPattern.String())
	}
	return nil
}
