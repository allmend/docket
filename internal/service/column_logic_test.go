package service

import (
	"errors"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestColumnNeighbour(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	// Deliberately out of order: board order comes from position, not slice order.
	cols := []model.Column{
		{ID: c, Position: 3000},
		{ID: a, Position: 1000},
		{ID: b, Position: 2000},
	}

	cases := []struct {
		name string
		id   uuid.UUID
		dir  int
		want uuid.UUID
		err  error
	}{
		{"middle moves left", b, -1, a, nil},
		{"middle moves right", b, 1, c, nil},
		{"first moves right", a, 1, b, nil},
		{"first cannot move left", a, -1, uuid.Nil, ErrInvalidInput},
		{"last cannot move right", c, 1, uuid.Nil, ErrInvalidInput},
		{"no direction", b, 0, uuid.Nil, ErrInvalidInput},
		{"column not on the board reads as missing", uuid.New(), 1, uuid.Nil, pgx.ErrNoRows},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := columnNeighbour(cols, tc.id, tc.dir)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("want %v, got %v", tc.err, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}
