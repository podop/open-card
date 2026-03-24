package secrets

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type RotationEngine struct {
	store        *Store
	handler      *Handler
	log          *zap.Logger
	ticker       *time.Ticker
	stop         chan struct{}
}

func NewRotationEngine(store *Store, handler *Handler, log *zap.Logger) *RotationEngine {
	return &RotationEngine{
		store:   store,
		handler: handler,
		log:     log,
		stop:    make(chan struct{}),
	}
}

func (e *RotationEngine) Start() {
	e.ticker = time.NewTicker(1 * time.Hour)
	go func() {
		e.log.Info("rotation engine started (1h interval)")
		// Run once immediately on start
		e.checkRotations()
		for {
			select {
			case <-e.ticker.C:
				e.checkRotations()
			case <-e.stop:
				e.ticker.Stop()
				e.log.Info("rotation engine stopped")
				return
			}
		}
	}()
}

func (e *RotationEngine) Stop() {
	close(e.stop)
}

func (e *RotationEngine) checkRotations() {
	ctx := context.Background()
	keys, err := e.store.GetKeysForRotation(ctx)
	if err != nil {
		e.log.Error("rotation check: query failed", zap.Error(err))
		return
	}

	for _, key := range keys {
		if !key.RotationPolicyID.Valid {
			continue
		}

		policy, err := e.store.GetPolicy(ctx, int(key.RotationPolicyID.Int32))
		if err != nil {
			e.log.Error("rotation check: policy lookup failed", zap.Int("keyID", key.ID), zap.Error(err))
			continue
		}

		if policy.AutoRotate {
			e.log.Info("auto-rotating key", zap.Int("keyID", key.ID), zap.String("name", key.Name), zap.Int("version", key.Version))
			_, err := e.handler.rotateKeyInternal(ctx, key.ID, NullInt32{}, "scheduled")
			if err != nil {
				e.log.Error("auto-rotation failed", zap.Int("keyID", key.ID), zap.Error(err))
			}
		} else {
			e.log.Warn("key due for rotation (auto_rotate=off)",
				zap.Int("keyID", key.ID),
				zap.String("name", key.Name),
				zap.Time("nextRotation", key.NextRotationAt.Time),
			)
		}
	}
}
