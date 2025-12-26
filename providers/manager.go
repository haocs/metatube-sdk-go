package providers

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Provider is a minimal interface the manager uses to validate an ID.
// Your existing provider implementations should implement this method.
type Provider interface {
	// ValidateProviderID returns (true, nil) if the provider_id is valid.
	// Return (false, nil) if the provider_id is known invalid.
	// Return (false, err) if there was an error while validating (transient).
	ValidateProviderID(ctx context.Context, providerID string) (bool, error)
}

// BlocklistChecker determines whether a provider name is blocklisted.
type BlocklistChecker func(providerName string) bool

// ProvidersMap maps provider name -> Provider implementation.
type ProvidersMap map[string]Provider

// ErrNoProviderForName indicates we don't have a provider implementation for a name.
var ErrNoProviderForName = errors.New("provider implementation not found")

// MaybeAutoClearProviderID will inspect providerID and, if the provider name is blocklisted
// and the provider reports the providerID is invalid, return an empty string (cleared).
//
// Behavior notes:
// - providerID format expected: "<providerName>:<rest>" (uses strings.SplitN with ":" separator).
//   If no ":" present, it returns providerID unchanged.
// - If the provider name is not blocklisted, providerID is returned unchanged.
// - If the provider is blocklisted but no Provider instance exists in providersMap, the ID is returned unchanged
//   and ErrNoProviderForName is returned. (This is safer than clearing on unknown implementation.)
// - If Provider.ValidateProviderID returns (false, nil) -> this indicates invalid -> function returns "".
// - If Provider.ValidateProviderID returns (false, err) -> on error the function returns providerID unchanged
//   to avoid clearing on transient errors. Adjust if you prefer different semantics.
func MaybeAutoClearProviderID(ctx context.Context, providerID string, checker BlocklistChecker, providersMap ProvidersMap) (string, error) {
	if providerID == "" {
		return "", nil
	}

	// fast parse: expect provider:rest
	parts := strings.SplitN(providerID, ":", 2)
	if len(parts) < 2 {
		// unknown format; do not touch
		return providerID, nil
	}
	providerName := parts[0]

	// if provider not blocklisted, do nothing
	if !checker(providerName) {
		return providerID, nil
	}

	// provider is blocklisted; try to validate the providerID
	p, ok := providersMap[providerName]
	if !ok || p == nil {
		// no provider implementation to validate with; safer to leave providerID alone
		return providerID, ErrNoProviderForName
	}

	// call validate, with a timeout to avoid long blocking
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	valid, err := p.ValidateProviderID(ctx, providerID)
	if err != nil {
		// transient error validating — do not clear provider_id automatically
		// Caller can decide further handling if it wants to clear on error.
		return providerID, err
	}

	if !valid {
		// provider is blocklisted AND providerID is invalid -> clear it so other providers can be tried
		return "", nil
	}

	// providerID appears valid -> do not clear
	return providerID, nil
}
