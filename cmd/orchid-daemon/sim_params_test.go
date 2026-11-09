package main

import "testing"

func TestValidateRemoteSimParams(t *testing.T) {
	// Defaults and in-range values are accepted: handler defaults (16384, 18),
	// the minimum (1, 0), and the exact caps.
	ok := []struct {
		n, trace int
	}{
		{16384, 18},
		{1, 0},
		{maxRemoteSimN, maxRemoteTraceLimit},
	}
	for _, c := range ok {
		if err := validateRemoteSimParams(c.n, c.trace); err != nil {
			t.Errorf("validateRemoteSimParams(%d, %d) = %v, want nil", c.n, c.trace, err)
		}
	}

	// Over-cap values are rejected.
	bad := []struct {
		name     string
		n, trace int
	}{
		{"n over cap", maxRemoteSimN + 1, 18},
		{"trace over cap", 16384, maxRemoteTraceLimit + 1},
		{"huge n", 1 << 40, 18},
	}
	for _, c := range bad {
		if err := validateRemoteSimParams(c.n, c.trace); err == nil {
			t.Errorf("%s: validateRemoteSimParams(%d, %d) = nil, want error", c.name, c.n, c.trace)
		}
	}
}

func TestValidateRemoteLocalityParams(t *testing.T) {
	for _, r := range []int{1, 8, maxRemoteLocalityRepeat} {
		if err := validateRemoteLocalityParams(r); err != nil {
			t.Errorf("validateRemoteLocalityParams(%d) = %v, want nil", r, err)
		}
	}
	for _, r := range []int{maxRemoteLocalityRepeat + 1, 1 << 30} {
		if err := validateRemoteLocalityParams(r); err == nil {
			t.Errorf("validateRemoteLocalityParams(%d) = nil, want error", r)
		}
	}
}
