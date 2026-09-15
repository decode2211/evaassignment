// Unit tests for the ticket status transition rules. These tests exist to
// make one guarantee obvious and permanent: once a ticket is closed, there
// is no code path - now or after any future change - that can move it
// anywhere else.
package models

import "testing"

func TestCanTransition(t *testing.T) {
	allStatuses := []Status{StatusOpen, StatusInProgress, StatusClosed}

	// allowed lists every (from, to) pair that must succeed. Every other
	// combination of the three statuses (there are 9 total pairs
	// including same-to-same) must fail.
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
