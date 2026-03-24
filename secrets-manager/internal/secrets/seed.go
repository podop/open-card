package secrets

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"
)

func Seed(ctx context.Context, store *Store, log *zap.Logger) error {
	// 1. Ensure "Internal" provider exists
	exists, err := store.ProviderExists(ctx, "Internal")
	if err != nil {
		return err
	}
	if !exists {
		log.Info("seeding Internal security provider")
		_, err := store.CreateProvider(ctx, &SecurityProvider{
			Name:         "Internal",
			ProviderType: "internal",
			Config:       json.RawMessage(`{}`),
			Active:       true,
			CreatedBy:    1,
		})
		if err != nil {
			return err
		}
	}

	// 2. Seed rotation policy templates
	templates := []RotationPolicy{
		{
			Name: "RSA-2048 Standard", Description: "Standard rotation for RSA-2048 keys",
			KeyType: "RSA-2048", RotationInterval: "2 years", NotifyBefore: "30 days",
			AutoRotate: false, IsTemplate: true, CreatedBy: 1,
		},
		{
			Name: "RSA-4096 Standard", Description: "Standard rotation for RSA-4096 keys",
			KeyType: "RSA-4096", RotationInterval: "3 years", NotifyBefore: "60 days",
			AutoRotate: false, IsTemplate: true, CreatedBy: 1,
		},
		{
			Name: "AES-256 Standard", Description: "Standard rotation for AES-256 keys",
			KeyType: "AES-256", RotationInterval: "1 year", NotifyBefore: "30 days",
			AutoRotate: true, IsTemplate: true, CreatedBy: 1,
		},
		{
			Name: "3DES Legacy", Description: "Rotation policy for legacy 3DES keys",
			KeyType: "3DES", RotationInterval: "6 months", NotifyBefore: "30 days",
			AutoRotate: true, IsTemplate: true, CreatedBy: 1,
		},
		{
			Name: "X509 Certificate", Description: "Rotation policy for X509 certificates",
			KeyType: "X509", RotationInterval: "1 year", NotifyBefore: "60 days",
			AutoRotate: false, IsTemplate: true, CreatedBy: 1,
		},
	}

	for _, t := range templates {
		exists, err := store.PolicyExists(ctx, t.Name)
		if err != nil {
			return err
		}
		if !exists {
			log.Info("seeding rotation policy template", zap.String("name", t.Name))
			if _, err := store.CreatePolicy(ctx, &t); err != nil {
				return err
			}
		}
	}

	log.Info("seed complete")
	return nil
}
