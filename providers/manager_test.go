package providers

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeProvider struct {
	validIDs map[string]bool
	errOn    bool
	delay    time.Duration
}

func (f *fakeProvider) ValidateProviderID(ctx context.Context, providerID string) (bool, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	if f.errOn {
		return false, errors.New("provider backend error")
	}
	return f.validIDs[providerID], nil
}

func TestMaybeAutoClearProviderID_notBlocklisted(t *testing.T) {
	ctx := context.Background()
	checker := func(name string) bool { return false } // nothing blocked
	providersMap := ProvidersMap{
		"xyz": &fakeProvider{validIDs: map[string]bool{"xyz:123": false}},
	}
	out, err := MaybeAutoClearProviderID(ctx, "xyz:123", checker, providersMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "xyz:123" {
		t.Fatalf("expected unchanged providerID, got %q", out)
	}
}

func TestMaybeAutoClearProviderID_blocklisted_invalid_clears(t *testing.T) {
	ctx := context.Background()
	checker := func(name string) bool { return name == "xyz" } // xyz blocked
	providersMap := ProvidersMap{
		"xyz": &fakeProvider{validIDs: map[string]bool{"xyz:123": false}},
	}
	out, err := MaybeAutoClearProviderID(ctx, "xyz:123", checker, providersMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected providerID cleared, got %q", out)
	}
}

func TestMaybeAutoClearProviderID_blocklisted_valid_keeps(t *testing.T) {
	ctx := context.Background()
	checker := func(name string) bool { return name == "xyz" } // xyz blocked
	providersMap := ProvidersMap{
		"xyz": &fakeProvider{validIDs: map[string]bool{"xyz:123": true}},
	}
	out, err := MaybeAutoClearProviderID(ctx, "xyz:123", checker, providersMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "xyz:123" {
		t.Fatalf("expected providerID unchanged, got %q", out)
	}
}

func TestMaybeAutoClearProviderID_blocklisted_validate_error_returns_original(t *testing.T) {
	ctx := context.Background()
	checker := func(name string) bool { return name == "xyz" } // xyz blocked
	providersMap := ProvidersMap{
		"xyz": &fakeProvider{errOn: true},
	}
	out, err := MaybeAutoClearProviderID(ctx, "xyz:123", checker, providersMap)
	if err == nil {
		t.Fatalf("expected error from provider validation")
	}
	if out != "xyz:123" {
		t.Fatalf("expected providerID unchanged on validation error, got %q", out)
	}
}

func TestMaybeAutoClearProviderID_blocklisted_provider_missing_returns_error(t *testing.T) {
	ctx := context.Background()
	checker := func(name string) bool { return name == "xyz" } // xyz blocked
	providersMap := ProvidersMap{}
	out, err := MaybeAutoClearProviderID(ctx, "xyz:123", checker, providersMap)
	if err == nil {
		t.Fatalf("expected ErrNoProviderForName")
	}
	if err != ErrNoProviderForName {
		t.Fatalf("expected ErrNoProviderForName, got %v", err)
	}
	if out != "xyz:123" {
		t.Fatalf("expected providerID unchanged when provider implementation missing, got %q", out)
	}
}
