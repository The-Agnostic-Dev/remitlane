// Package domain holds the payout vocabulary shared by every layer.
package domain

// Status is a payout's lifecycle state.
type Status string

// Payout statuses. A payout starts pending, is submitted to a provider, may
// report processing, and ends completed or failed. A completed payout can
// later be reversed by the recipient's bank.
const (
	Pending    Status = "pending"
	Submitted  Status = "submitted"
	Processing Status = "processing"
	Completed  Status = "completed"
	Failed     Status = "failed"
	Reversed   Status = "reversed"
)

var transitions = map[Status][]Status{
	Pending:    {Submitted, Processing, Completed, Failed},
	Submitted:  {Processing, Completed, Failed},
	Processing: {Completed, Failed},
	Completed:  {Reversed},
}

// CanMove reports whether a payout may go from one status to another.
func CanMove(from, to Status) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Terminal reports whether no further provider update is expected.
func (s Status) Terminal() bool { return s == Failed || s == Reversed }

// Final reports whether the payout has reached an end state (a completed
// payout may still be reversed, but it is final for limits and reports).
func (s Status) Final() bool { return s == Completed || s == Failed || s == Reversed }

// AllStatuses lists statuses in lifecycle order, for filters.
func AllStatuses() []Status {
	return []Status{Pending, Submitted, Processing, Completed, Failed, Reversed}
}

// EventType is the partner webhook event for a status, or "" when the
// status change is not announced.
func (s Status) EventType() string {
	switch s {
	case Processing:
		return "payout.processing"
	case Completed:
		return "payout.completed"
	case Failed:
		return "payout.failed"
	case Reversed:
		return "payout.reversed"
	}
	return ""
}
