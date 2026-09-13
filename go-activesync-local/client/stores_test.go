package client

import (
	"context"
	"testing"
)

// SPEC: MS-ASHTTP/store.policy
func TestInMemoryPolicyStore(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryPolicyStore()
	if k, err := s.Get(ctx); err != nil || k != "" {
		t.Fatalf("initial Get = %q, %v", k, err)
	}
	if err := s.Set(ctx, "12345"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if k, _ := s.Get(ctx); k != "12345" {
		t.Errorf("Get after Set = %q", k)
	}
}

// SPEC: MS-ASHTTP/store.syncstate
func TestInMemorySyncStateStore(t *testing.T) {
	ctx := context.Background()
	s := NewInMemorySyncStateStore()
	k, err := s.Get(ctx, "Inbox")
	if err != nil || k != "0" {
		t.Fatalf("initial Inbox Get = %q (err=%v); want \"0\"", k, err)
	}
	if err := s.Set(ctx, "Inbox", "abc"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if k, _ := s.Get(ctx, "Inbox"); k != "abc" {
		t.Errorf("Get after Set = %q", k)
	}
	// Distinct collections do not share state.
	if k, _ := s.Get(ctx, "Calendar"); k != "0" {
		t.Errorf("Calendar still default = %q", k)
	}
}
