package service

import (
	"fmt"
	"sort"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// columnNeighbour returns the column next to columnID in board order: the one
// before it for dir < 0, after it for dir > 0. A step past either end is
// invalid input; a column that isn't on the board is pgx.ErrNoRows (404).
func columnNeighbour(cols []model.Column, columnID uuid.UUID, dir int) (uuid.UUID, error) {
	if dir == 0 {
		return uuid.Nil, fmt.Errorf("%w: direction must be left or right", ErrInvalidInput)
	}
	ordered := append([]model.Column(nil), cols...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Position < ordered[j].Position })

	for i, c := range ordered {
		if c.ID != columnID {
			continue
		}
		j := i + 1
		if dir < 0 {
			j = i - 1
		}
		if j < 0 || j >= len(ordered) {
			return uuid.Nil, fmt.Errorf("%w: the column is already at that end", ErrInvalidInput)
		}
		return ordered[j].ID, nil
	}
	// Not on this board (or not in this org): reads exactly like a missing ID.
	return uuid.Nil, pgx.ErrNoRows
}
