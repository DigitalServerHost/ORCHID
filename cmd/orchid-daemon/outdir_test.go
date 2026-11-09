package main

import "testing"

func TestValidateRemoteOutDir(t *testing.T) {
	allowed := []string{
		"evidence",
		"evidence/current",
		"evidence/reproduced",
		"evidence/custom/run-1",
		"./evidence/current",
		"evidence/current/../reproduced",
	}
	for _, dir := range allowed {
		if err := validateRemoteOutDir(dir); err != nil {
			t.Errorf("validateRemoteOutDir(%q) = %v, want nil", dir, err)
		}
	}

	rejected := []string{
		"/etc",
		"/tmp/evidence",
		"../evidence",
		"evidence/../../etc",
		"evidence/../cmd",
		"evidencex",
		"build",
		"",
		".",
	}
	for _, dir := range rejected {
		if err := validateRemoteOutDir(dir); err == nil {
			t.Errorf("validateRemoteOutDir(%q) = nil, want error", dir)
		}
	}
}
