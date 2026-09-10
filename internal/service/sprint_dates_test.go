package service

import (
	"errors"
	"testing"
	"time"
)

func TestValidateSprintDates(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 14)

	if err := validateSprintDates(&start, &end); err != nil {
		t.Errorf("end after start: unexpected %v", err)
	}
	if err := validateSprintDates(&start, &start); err != nil {
		t.Errorf("same day: unexpected %v", err)
	}
	if err := validateSprintDates(nil, &end); err != nil {
		t.Errorf("nil start: unexpected %v", err)
	}
	if err := validateSprintDates(&end, &start); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("end before start: want ErrInvalidInput, got %v", err)
	}
}
