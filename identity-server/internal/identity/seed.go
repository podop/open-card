package identity

import (
	"context"

	"github.com/your-org/identity-server/internal/actions"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Seed initialises the identity tables on first boot:
// RSA key pair, default settings, admin user, Administrators group.
func Seed(ctx context.Context, store *Store, log *zap.Logger) error {
	exists, err := store.UserExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		// Ensure Administrators group has all current actions (covers newly added actions).
		if err := ensureAdminActions(ctx, store, log); err != nil {
			log.Warn("failed to reconcile admin group actions", zap.Error(err))
		}
		log.Info("identity data already seeded, skipping")
		return nil
	}

	log.Info("seeding identity data (first boot)")

	// Generate RSA key pair
	privPEM, pubPEM, err := GenerateRSAKeyPair()
	if err != nil {
		return err
	}
	if err := store.SetSetting(ctx, "rsa_private_key", privPEM); err != nil {
		return err
	}
	if err := store.SetSetting(ctx, "rsa_public_key", pubPEM); err != nil {
		return err
	}
	if err := store.SetSetting(ctx, "token_expiry_minutes", "60"); err != nil {
		return err
	}
	if err := store.SetSetting(ctx, "refresh_expiry_hours", "720"); err != nil {
		return err
	}
	if err := store.SetSetting(ctx, "active", "true"); err != nil {
		return err
	}

	// Create admin user
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	userID, err := store.CreateUser(ctx, "admin", string(hash), "Administrator")
	if err != nil {
		return err
	}

	// Create Administrators group with all actions
	groupID, err := store.CreateGroup(ctx, "Administrators", "Full access to all operations", actions.AllActions)
	if err != nil {
		return err
	}

	// Link admin → Administrators
	if err := store.SetUserGroups(ctx, userID, []int{groupID}); err != nil {
		return err
	}

	log.Info("identity seed complete", zap.Int("adminUserID", userID), zap.Int("adminGroupID", groupID))
	return nil
}

// ensureAdminActions updates the Administrators group (ID=1) to include all current actions.
func ensureAdminActions(ctx context.Context, store *Store, log *zap.Logger) error {
	g, err := store.GetGroup(ctx, 1)
	if err != nil {
		return err
	}
	if g == nil || g.Name != "Administrators" {
		return nil
	}

	existing := make(map[string]bool)
	for _, a := range g.Actions {
		existing[a] = true
	}
	var missing []string
	for _, a := range actions.AllActions {
		if !existing[a] {
			missing = append(missing, a)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	log.Info("adding missing actions to Administrators group", zap.Strings("actions", missing))
	return store.UpdateGroup(ctx, g.ID, g.Name, g.Description, actions.AllActions)
}
