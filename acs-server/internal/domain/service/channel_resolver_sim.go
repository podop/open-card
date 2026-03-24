package service

import (
	"context"
	"sync"

	"github.com/your-org/acs-server/internal/domain/entity"
)

// SimulatedOTPResolver is a thread-safe, runtime-togglable OTP channel resolver
// for testing and simulation purposes.
type SimulatedOTPResolver struct {
	mu        sync.RWMutex
	available bool
}

// NewSimulatedOTPResolver creates a new simulated OTP resolver.
func NewSimulatedOTPResolver(available bool) *SimulatedOTPResolver {
	return &SimulatedOTPResolver{available: available}
}

func (r *SimulatedOTPResolver) CheckOTPAvailable(_ context.Context, _ string, _ entity.Network) (*ChannelAvailability, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.available {
		return &ChannelAvailability{Available: true, Reason: "simulated: available"}, nil
	}
	return &ChannelAvailability{Available: false, Reason: "simulated: unavailable"}, nil
}

// SetAvailable toggles OTP availability at runtime.
func (r *SimulatedOTPResolver) SetAvailable(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.available = v
}

// IsAvailable returns the current availability setting.
func (r *SimulatedOTPResolver) IsAvailable() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.available
}

// SimulatedOOBResolver is a thread-safe, runtime-togglable OOB channel resolver
// for testing and simulation purposes.
type SimulatedOOBResolver struct {
	mu        sync.RWMutex
	available bool
}

// NewSimulatedOOBResolver creates a new simulated OOB resolver.
func NewSimulatedOOBResolver(available bool) *SimulatedOOBResolver {
	return &SimulatedOOBResolver{available: available}
}

func (r *SimulatedOOBResolver) CheckOOBAvailable(_ context.Context, _ string, _ entity.Network) (*ChannelAvailability, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.available {
		return &ChannelAvailability{Available: true, Reason: "simulated: available"}, nil
	}
	return &ChannelAvailability{Available: false, Reason: "simulated: unavailable"}, nil
}

// SetAvailable toggles OOB availability at runtime.
func (r *SimulatedOOBResolver) SetAvailable(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.available = v
}

// IsAvailable returns the current availability setting.
func (r *SimulatedOOBResolver) IsAvailable() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.available
}
