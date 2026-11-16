package main

import (
	"context"
	"sync"

	"github.com/Adis/steamlib-explorer/backend/internal/settings"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
)

// keySetup lets the setup page set the Steam API key: it saves the key to
// settings.json, then the running Steam client starts using it.
type keySetup struct {
	mu     sync.Mutex
	client *steam.Client
	file   string
}

func (k *keySetup) HasAPIKey() bool { return k.client.APIKey() != "" }

func (k *keySetup) CheckAPIKey(ctx context.Context, key string) error {
	return k.client.CheckAPIKey(ctx, key)
}

func (k *keySetup) SaveAPIKey(key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	s, err := settings.Load(k.file)
	if err != nil {
		return err
	}
	s.SteamAPIKey = key
	if err := settings.Save(k.file, s); err != nil {
		return err
	}
	k.client.SetAPIKey(key)
	return nil
}
