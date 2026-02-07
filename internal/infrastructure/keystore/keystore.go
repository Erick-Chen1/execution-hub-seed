package keystore

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

// StaticKeyStore is a simple in-memory keystore.
type StaticKeyStore struct {
	keys          map[string][]byte
	defaultKeyID  string
	perSourceKeys map[string]string
}

// NewFromEnv builds a keystore from environment variables.
// SIGNING_KEYS format: "keyId:hex,keyId2:hex".
// SIGNING_DEFAULT_KEY_ID sets the default key id.
// SIGNING_KEY_FOR_SOURCE_<sourceId> can override key per source.
func NewFromEnv() (*StaticKeyStore, error) {
	keys := make(map[string][]byte)
	raw := os.Getenv("SIGNING_KEYS")
	if raw != "" {
		pairs := strings.Split(raw, ",")
		for _, p := range pairs {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			parts := strings.SplitN(p, ":", 2)
			if len(parts) != 2 {
				return nil, errors.New("invalid SIGNING_KEYS format")
			}
			keyID := parts[0]
			bytes, err := hex.DecodeString(parts[1])
			if err != nil {
				return nil, err
			}
			keys[keyID] = bytes
		}
	}

	ks := &StaticKeyStore{
		keys:          keys,
		defaultKeyID:  os.Getenv("SIGNING_DEFAULT_KEY_ID"),
		perSourceKeys: map[string]string{},
	}

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "SIGNING_KEY_FOR_SOURCE_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}
			src := strings.TrimPrefix(parts[0], "SIGNING_KEY_FOR_SOURCE_")
			if src != "" {
				ks.perSourceKeys[src] = parts[1]
			}
		}
	}

	return ks, nil
}

func (s *StaticKeyStore) GetKey(ctx context.Context, keyID string) ([]byte, error) {
	_ = ctx
	key, ok := s.keys[keyID]
	if !ok {
		return nil, errors.New("key not found")
	}
	return key, nil
}

func (s *StaticKeyStore) GetKeyForSource(ctx context.Context, sourceID string) (keyID string, key []byte, err error) {
	_ = ctx
	if srcKeyID, ok := s.perSourceKeys[sourceID]; ok && srcKeyID != "" {
		key, err = s.GetKey(context.Background(), srcKeyID)
		return srcKeyID, key, err
	}
	if s.defaultKeyID == "" {
		return "", nil, errors.New("default key not configured")
	}
	key, err = s.GetKey(context.Background(), s.defaultKeyID)
	return s.defaultKeyID, key, err
}
