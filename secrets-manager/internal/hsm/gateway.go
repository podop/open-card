package hsm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// GatewayManager orchestrates HSM connection pools, health checks, and load balancing.
type GatewayManager struct {
	store    *Store
	log      *zap.Logger
	mu       sync.RWMutex
	pools    map[int]*ConnectionPool
	states   map[int]*EndpointState
	failures map[int]int // consecutive health check failures per endpoint
	strategy BalancingStrategy
	ticker   *time.Ticker
	stop     chan struct{}
}

func NewGatewayManager(store *Store, log *zap.Logger) *GatewayManager {
	return &GatewayManager{
		store:    store,
		log:      log,
		pools:    make(map[int]*ConnectionPool),
		states:   make(map[int]*EndpointState),
		failures: make(map[int]int),
		strategy: &CapabilityStrategy{}, // default strategy
		stop:     make(chan struct{}),
	}
}

// Start loads endpoints from the database, creates pools, and begins health checks.
func (g *GatewayManager) Start() {
	ctx := context.Background()
	endpoints, err := g.store.GetActiveEndpoints(ctx)
	if err != nil {
		g.log.Error("HSM Gateway: failed to load endpoints", zap.Error(err))
	} else {
		for _, ep := range endpoints {
			g.addPool(ep)
		}
		g.log.Info("HSM Gateway: loaded endpoints", zap.Int("count", len(endpoints)))
	}

	g.ticker = time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-g.ticker.C:
				g.runHealthChecks()
			case <-g.stop:
				return
			}
		}
	}()
	g.log.Info("HSM Gateway: started")
}

// Stop gracefully shuts down all connection pools.
func (g *GatewayManager) Stop() {
	close(g.stop)
	if g.ticker != nil {
		g.ticker.Stop()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for id, pool := range g.pools {
		g.log.Info("HSM Gateway: closing pool", zap.Int("endpointId", id))
		pool.Close()
	}
	g.pools = make(map[int]*ConnectionPool)
	g.states = make(map[int]*EndpointState)
	g.log.Info("HSM Gateway: stopped")
}

// SetStrategy changes the global balancing strategy.
func (g *GatewayManager) SetStrategy(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.strategy = NewStrategy(name)
	g.log.Info("HSM Gateway: strategy changed", zap.String("strategy", name))
}

// RouteRequest selects an endpoint based on the balancing strategy and required capability.
func (g *GatewayManager) RouteRequest(ctx context.Context, capability string) (*ConnectionPool, int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var available []*EndpointState
	for _, state := range g.states {
		if state.Status == "online" || state.Status == "degraded" {
			available = append(available, state)
		}
	}
	if len(available) == 0 {
		return nil, 0, fmt.Errorf("no HSM endpoints available")
	}

	selected := g.strategy.Select(available, capability)
	if selected == nil {
		return nil, 0, fmt.Errorf("no HSM endpoint supports capability: %s", capability)
	}

	return selected.Pool, selected.ID, nil
}

// AddEndpoint registers a new endpoint and creates its connection pool.
func (g *GatewayManager) AddEndpoint(ep HSMEndpoint) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.addPool(ep)
	g.log.Info("HSM Gateway: endpoint added", zap.Int("id", ep.ID), zap.String("name", ep.Name))
}

// RemoveEndpoint removes an endpoint and closes its pool.
func (g *GatewayManager) RemoveEndpoint(id int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if pool, ok := g.pools[id]; ok {
		pool.Close()
		delete(g.pools, id)
		delete(g.states, id)
		delete(g.failures, id)
	}
	g.log.Info("HSM Gateway: endpoint removed", zap.Int("id", id))
}

// DrainEndpoint puts an endpoint into draining state.
func (g *GatewayManager) DrainEndpoint(id int) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	pool, ok := g.pools[id]
	if !ok {
		return fmt.Errorf("endpoint %d not found in gateway", id)
	}
	pool.Drain()
	if state, ok := g.states[id]; ok {
		state.Status = "draining"
	}
	ctx := context.Background()
	g.store.UpdateEndpointStatus(ctx, id, "draining", true, NullString{}) //nolint:errcheck
	g.log.Info("HSM Gateway: endpoint draining", zap.Int("id", id))
	return nil
}

// ResumeEndpoint resumes a drained endpoint.
func (g *GatewayManager) ResumeEndpoint(id int) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	pool, ok := g.pools[id]
	if !ok {
		return fmt.Errorf("endpoint %d not found in gateway", id)
	}
	pool.Resume()
	if state, ok := g.states[id]; ok {
		state.Status = "online"
	}
	ctx := context.Background()
	g.store.UpdateEndpointStatus(ctx, id, "online", true, NullString{}) //nolint:errcheck
	g.log.Info("HSM Gateway: endpoint resumed", zap.Int("id", id))
	return nil
}

// ReconnectEndpoint closes and recreates the pool for an endpoint.
func (g *GatewayManager) ReconnectEndpoint(id int) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	pool, ok := g.pools[id]
	if !ok {
		return fmt.Errorf("endpoint %d not found in gateway", id)
	}
	ep := pool.Endpoint()
	pool.Close()
	delete(g.pools, id)
	delete(g.states, id)
	g.failures[id] = 0
	g.addPool(ep)
	g.log.Info("HSM Gateway: endpoint reconnected", zap.Int("id", id))
	return nil
}

// GetEndpointStatus returns the runtime status of an endpoint.
func (g *GatewayManager) GetEndpointStatus(id int) (*EndpointStatus, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	pool, ok := g.pools[id]
	if !ok {
		return nil, false
	}
	ep := pool.Endpoint()
	return &EndpointStatus{
		HSMEndpoint: ep,
		Pool:        pool.Stats(),
	}, true
}

// ListEndpointStatuses returns all endpoint statuses with pool stats.
func (g *GatewayManager) ListEndpointStatuses() []EndpointStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []EndpointStatus
	for _, pool := range g.pools {
		ep := pool.Endpoint()
		if state, ok := g.states[ep.ID]; ok {
			ep.Status = state.Status
		}
		result = append(result, EndpointStatus{
			HSMEndpoint: ep,
			Pool:        pool.Stats(),
		})
	}
	return result
}

// SendCommand sends a command to a specific endpoint by ID.
func (g *GatewayManager) SendCommand(ctx context.Context, endpointID int, cmdCode string, data []byte) ([]byte, error) {
	g.mu.RLock()
	pool, ok := g.pools[endpointID]
	g.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("endpoint %d not found in gateway", endpointID)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer pool.Release(conn)

	timeout := time.Duration(pool.Endpoint().HostCommandTimeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return conn.SendCommand(cmdCode, data, timeout)
}

func (g *GatewayManager) addPool(ep HSMEndpoint) {
	pool := NewPool(ep, g.log)
	g.pools[ep.ID] = pool
	g.states[ep.ID] = &EndpointState{
		ID:           ep.ID,
		Weight:       ep.Weight,
		Capabilities: []string(ep.Capabilities),
		Status:       ep.Status,
		Pool:         pool,
	}
}

func (g *GatewayManager) runHealthChecks() {
	g.mu.RLock()
	var toCheck []int
	for id := range g.pools {
		toCheck = append(toCheck, id)
	}
	g.mu.RUnlock()

	for _, id := range toCheck {
		g.mu.RLock()
		pool, ok := g.pools[id]
		g.mu.RUnlock()
		if !ok {
			continue
		}

		err := pool.HealthCheck()
		g.mu.Lock()
		ctx := context.Background()
		if err != nil {
			g.failures[id]++
			failures := g.failures[id]
			var newStatus string
			if failures >= 3 {
				newStatus = "offline"
			} else {
				newStatus = "degraded"
			}
			if state, ok := g.states[id]; ok {
				state.Status = newStatus
			}
			g.store.UpdateEndpointStatus(ctx, id, newStatus, false, //nolint:errcheck
				NullString{String: err.Error(), Valid: true})
			g.log.Warn("HSM health check failed",
				zap.Int("endpointId", id),
				zap.Int("failures", failures),
				zap.String("status", newStatus),
				zap.Error(err))
		} else {
			g.failures[id] = 0
			if state, ok := g.states[id]; ok && state.Status != "draining" {
				state.Status = "online"
			}
			g.store.UpdateEndpointStatus(ctx, id, "online", true, NullString{}) //nolint:errcheck
		}
		g.mu.Unlock()
	}
}
