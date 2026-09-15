// Tests for the ticket status transition rules.
package models

import "testing"

func TestCanTransition(t *testing.T) {
	allStatuses := []Status{StatusOpen, StatusInProgress, StatusClosed}

	// Every (from, to) pair not listed here must fail.
	allowed := map[Status]map[Status]bool{
		StatusOpen:       {StatusInProgress: true, StatusClosed: true},
		StatusInProgress: {StatusClosed: true},
		StatusClosed:     {},
	}

	for _, from := range allStatuses {
		for _, to := range allStatuses {
			wantOK := allowed[from][to]
			err := CanTransition(from, to)
			gotOK := err == nil

			if gotOK != wantOK {
				t.Errorf("CanTransition(%q, %q) = %v, want ok=%v", from, to, err, wantOK)
			}
		}
	}
}

func TestCanTransition_ClosedNeverReopens(t *testing.T) {
	for _, to := range []Status{StatusOpen, StatusInProgress, StatusClosed} {
		if err := CanTransition(StatusClosed, to); err == nil {
			t.Errorf("CanTransition(closed, %q) succeeded, want error", to)
		}
	}
}

func TestCanTransition_InvalidStatusValues(t *testing.T) {
	if err := CanTransition(Status("bogus"), StatusOpen); err == nil {
		t.Error("expected error for invalid 'from' status, got nil")
	}
	if err := CanTransition(StatusOpen, Status("bogus")); err == nil {
		t.Error("expected error for invalid 'to' status, got nil")
	}
}

func TestStatus_IsValid(t *testing.T) {
	valid := []Status{StatusOpen, StatusInProgress, StatusClosed}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	invalid := []Status{"", "OPEN", "pending", "reopened"}
	for _, s := range invalid {
		if s.IsValid() {
			t.Errorf("%q should not be valid", s)
		}
	}
}
