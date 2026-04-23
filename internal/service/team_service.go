package service

import (
	"context"
	"strings"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

type TeamService struct {
	store  *store.Store
	boards *BoardService
}

func NewTeamService(st *store.Store, boards *BoardService) *TeamService {
	return &TeamService{store: st, boards: boards}
}

func (s *TeamService) ListTeams(ctx context.Context, orgID uuid.UUID) ([]model.Team, error) {
	return s.store.ListTeams(ctx, orgID)
}

func (s *TeamService) ListTeamsWithBoards(ctx context.Context, orgID uuid.UUID) ([]model.TeamWithBoard, error) {
	return s.store.ListTeamsWithBoards(ctx, orgID)
}

func (s *TeamService) GetTeam(ctx context.Context, orgID, teamID uuid.UUID) (*model.Team, error) {
	return s.store.GetTeam(ctx, orgID, teamID)
}

func (s *TeamService) GetTeamByKey(ctx context.Context, orgID uuid.UUID, key string) (*model.Team, error) {
	return s.store.GetTeamByKey(ctx, orgID, key)
}

// CreateTeam creates a team and immediately provisions its board.
// The board mode is chosen at team-creation time.
func (s *TeamService) CreateTeam(ctx context.Context, orgID, userID uuid.UUID, name, key, description string, mode model.BoardMode) (*model.Team, *model.Board, error) {
	team, err := s.store.CreateTeam(ctx, orgID, userID, name, NormaliseKey(key), description)
	if err != nil {
		return nil, nil, err
	}

	board, err := s.boards.CreateBoard(ctx, orgID, userID, &team.ID, name, "", mode)
	if err != nil {
		return nil, nil, err
	}

	return team, board, nil
}

func (s *TeamService) UpdateTeam(ctx context.Context, orgID, teamID uuid.UUID, name, description string) (*model.Team, error) {
	return s.store.UpdateTeam(ctx, orgID, teamID, name, description)
}

func (s *TeamService) DeleteTeam(ctx context.Context, orgID, teamID uuid.UUID) error {
	return s.store.DeleteTeam(ctx, orgID, teamID)
}

func (s *TeamService) GetBoardForTeam(ctx context.Context, orgID, teamID uuid.UUID) (*model.Board, error) {
	return s.store.GetBoardByTeamID(ctx, orgID, teamID)
}

func (s *TeamService) ListTeamMembers(ctx context.Context, orgID, teamID uuid.UUID) ([]model.User, error) {
	return s.store.ListTeamMembers(ctx, orgID, teamID)
}

func (s *TeamService) AddTeamMember(ctx context.Context, orgID, teamID, userID uuid.UUID) error {
	return s.store.AddTeamMember(ctx, orgID, teamID, userID)
}

func (s *TeamService) RemoveTeamMember(ctx context.Context, orgID, teamID, userID uuid.UUID) error {
	return s.store.RemoveTeamMember(ctx, orgID, teamID, userID)
}

func (s *TeamService) SearchNonMembers(ctx context.Context, orgID, teamID uuid.UUID, q string) ([]model.User, error) {
	return s.store.SearchNonMembers(ctx, orgID, teamID, q)
}

// NormaliseKey uppercases and strips non-alphanumeric characters from a team key.
func NormaliseKey(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(raw) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
