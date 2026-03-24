package hsm

import (
	"math/rand"
	"sync/atomic"
)

// EndpointState holds runtime state for balancing decisions.
type EndpointState struct {
	ID           int
	Weight       int
	Capabilities []string
	Status       string
	Pool         *ConnectionPool
}

// BalancingStrategy selects an endpoint from a set of candidates.
type BalancingStrategy interface {
	Select(endpoints []*EndpointState, capability string) *EndpointState
}

// ── Round Robin ─────────────────────────────────────────────────

type RoundRobinStrategy struct {
	counter atomic.Uint64
}

func (s *RoundRobinStrategy) Select(endpoints []*EndpointState, _ string) *EndpointState {
	if len(endpoints) == 0 {
		return nil
	}
	idx := s.counter.Add(1) - 1
	return endpoints[idx%uint64(len(endpoints))]
}

// ── Capability-Based ────────────────────────────────────────────

type CapabilityStrategy struct {
	rr RoundRobinStrategy
}

func (s *CapabilityStrategy) Select(endpoints []*EndpointState, capability string) *EndpointState {
	if capability == "" {
		return s.rr.Select(endpoints, capability)
	}
	var capable []*EndpointState
	for _, ep := range endpoints {
		if HasCapability(ep.Capabilities, capability) {
			capable = append(capable, ep)
		}
	}
	if len(capable) == 0 {
		return nil
	}
	return s.rr.Select(capable, capability)
}

// ── Weighted Random ─────────────────────────────────────────────

type WeightedStrategy struct{}

func (s *WeightedStrategy) Select(endpoints []*EndpointState, capability string) *EndpointState {
	var candidates []*EndpointState
	if capability != "" {
		for _, ep := range endpoints {
			if HasCapability(ep.Capabilities, capability) {
				candidates = append(candidates, ep)
			}
		}
	} else {
		candidates = endpoints
	}
	if len(candidates) == 0 {
		return nil
	}

	totalWeight := 0
	for _, ep := range candidates {
		w := ep.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	r := rand.Intn(totalWeight) //nolint:gosec
	cumulative := 0
	for _, ep := range candidates {
		w := ep.Weight
		if w <= 0 {
			w = 1
		}
		cumulative += w
		if r < cumulative {
			return ep
		}
	}
	return candidates[len(candidates)-1]
}

// NewStrategy creates a balancing strategy by name.
func NewStrategy(name string) BalancingStrategy {
	switch name {
	case "weighted":
		return &WeightedStrategy{}
	case "capability":
		return &CapabilityStrategy{}
	default:
		return &RoundRobinStrategy{}
	}
}
