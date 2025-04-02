package domain

import "testing"

func TestTransitions(t *testing.T) {
	ok := [][2]Status{
		{Pending, Submitted}, {Submitted, Processing}, {Processing, Completed},
		{Pending, Completed}, {Completed, Reversed}, {Submitted, Failed},
	}
	for _, c := range ok {
		if !CanMove(c[0], c[1]) {
			t.Errorf("%s -> %s should be allowed", c[0], c[1])
		}
	}
	bad := [][2]Status{
		{Failed, Completed}, {Completed, Failed}, {Reversed, Completed}, {Processing, Pending},
	}
	for _, c := range bad {
		if CanMove(c[0], c[1]) {
			t.Errorf("%s -> %s should be refused", c[0], c[1])
		}
	}
}

func TestEventType(t *testing.T) {
	if Completed.EventType() != "payout.completed" || Pending.EventType() != "" {
		t.Error("event types")
	}
}
