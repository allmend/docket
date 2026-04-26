package service

import (
	"context"

	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

type MetricsService struct {
	store *store.Store
}

func NewMetricsService(st *store.Store) *MetricsService {
	return &MetricsService{store: st}
}

func (s *MetricsService) TicketCounts(ctx context.Context, orgID uuid.UUID) ([]store.TicketCountRow, error) {
	return s.store.MetricsTicketCounts(ctx, orgID)
}

func (s *MetricsService) BacklogSize(ctx context.Context, orgID uuid.UUID) ([]store.BacklogSizeRow, error) {
	return s.store.MetricsBacklogSize(ctx, orgID)
}

func (s *MetricsService) BlockedCount(ctx context.Context, orgID uuid.UUID) ([]store.BlockedCountRow, error) {
	return s.store.MetricsBlockedCount(ctx, orgID)
}

func (s *MetricsService) SprintStats(ctx context.Context, orgID uuid.UUID) ([]store.SprintStatsRow, error) {
	return s.store.MetricsSprintStats(ctx, orgID)
}
