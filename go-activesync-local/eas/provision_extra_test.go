package eas

import "testing"

// SPEC: MS-ASPROV/two-phase-flow
func TestNewInitialRequest(t *testing.T) {
	r := NewInitialRequest()
	if len(r.Policies.Policy) != 1 {
		t.Fatalf("Policy count = %d", len(r.Policies.Policy))
	}
	p := r.Policies.Policy[0]
	if p.PolicyType != PolicyTypeWBXML {
		t.Errorf("PolicyType = %q", p.PolicyType)
	}
	if p.PolicyKey != "" || p.Status != 0 {
		t.Errorf("initial request must not carry PolicyKey/Status")
	}
}
