package app

import (
	"context"
	"log/slog"

	"github.com/Adis/steamlib-explorer/backend/internal/apikey"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
)

// keySetup lets the setup page set the Steam API key: it saves the key in the
// OS keychain (or settings.json without one), then the running Steam client
// starts using it.
type keySetup struct {
	client *steam.Client
	keys   *apikey.Store
}

func (k *keySetup) HasAPIKey() bool { return k.client.APIKey() != "" }

func (k *keySetup) CheckAPIKey(ctx context.Context, key string) error {
	return k.client.CheckAPIKey(ctx, key)
}

func (k *keySetup) SaveAPIKey(key string) error {
	where, err := k.keys.Save(key)
	if err != nil {
		return err
	}
	slog.Info("Steam API key saved", "in", where)
	k.client.SetAPIKey(key)
	return nil
}
