package ipam

import (
	"errors"
	"testing"
)

func TestAllocateNextIPAndReuse(t *testing.T) {
	subnet := "10.10.0.0/29" // .0 network, .1 gw, .2-.6 usable (5 hosts), .7 broadcast
	gw := "10.10.0.1"

	// First allocation should be .2
	ip1, err := AllocateNextIP(subnet, gw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip1 != "10.10.0.2" {
		t.Fatalf("expected 10.10.0.2, got %s", ip1)
	}

	// Second allocation with .2 used should be .3
	ip2, err := AllocateNextIP(subnet, gw, []string{"10.10.0.2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip2 != "10.10.0.3" {
		t.Fatalf("expected 10.10.0.3, got %s", ip2)
	}

	// Simulate .2 removed while .3 and .4 remain active -> .2 must be reused!
	reused, err := AllocateNextIP(subnet, gw, []string{"10.10.0.3", "10.10.0.4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reused != "10.10.0.2" {
		t.Fatalf("expected 10.10.0.2 to be reused, got %s", reused)
	}

	// Exhaust all 5 usable hosts (.2, .3, .4, .5, .6)
	_, err = AllocateNextIP(subnet, gw, []string{
		"10.10.0.2",
		"10.10.0.3",
		"10.10.0.4",
		"10.10.0.5",
		"10.10.0.6",
	})
	if !errors.Is(err, ErrSubnetExhausted) {
		t.Fatalf("expected ErrSubnetExhausted, got %v", err)
	}
}
