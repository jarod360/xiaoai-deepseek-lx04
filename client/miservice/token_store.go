// Modified or added for XiaoSeek LX04, 2026-10-24. See NOTICE.
// SPDX-License-Identifier: MIT
package miservice

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type SidToken struct {
	Ssecurity    string `json:"ssecurity"`
	ServiceToken string `json:"service_token"`
}

type Tokens struct {
	UserName  string              `json:"user_name"`
	DeviceId  string              `json:"device_id"`
	UserId    string              `json:"user_id"`
	PassToken string              `json:"pass_token"`
	Sids      map[string]SidToken `json:"sids"`
}

func NewTokens() *Tokens {
	return &Tokens{
		Sids: make(map[string]SidToken),
	}
}

type TokenStore interface {
	LoadToken() (*Tokens, error)
	SaveToken(tokens *Tokens) error
}

type FileTokenStore struct {
	tokenPath string
}

func NewTokenStore(tokenPath string) *FileTokenStore {
	return &FileTokenStore{tokenPath: tokenPath}
}

func (mts *FileTokenStore) LoadToken() (*Tokens, error) {
	var tokens Tokens
	if _, err := os.Stat(mts.tokenPath); os.IsNotExist(err) {
		return nil, err
	}
	data, err := os.ReadFile(mts.tokenPath)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &tokens)
	return &tokens, err
}

func (mts *FileTokenStore) SaveToken(tokens *Tokens) error {
	var err error
	if tokens != nil {
		data, err := json.MarshalIndent(tokens, "", "  ")
		if err != nil {
			return err
		}
		// Preserve the previous token on power loss or interrupted writes.
		tmp, err := os.CreateTemp(filepath.Dir(mts.tokenPath), ".mi-token-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if err := tmp.Chmod(0600); err != nil {
			tmp.Close()
			return err
		}
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			return err
		}
		if err := tmp.Sync(); err != nil {
			tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		return os.Rename(tmp.Name(), mts.tokenPath)
	} else {
		err = os.Remove(mts.tokenPath)
		if os.IsNotExist(err) {
			err = nil
		}
	}
	return err
}

type DummyTokenStore struct {
	tokens *Tokens
}

func (d *DummyTokenStore) LoadToken() (*Tokens, error) {
	return d.tokens, nil
}

func (d *DummyTokenStore) SaveToken(tokens *Tokens) error {
	d.tokens = tokens
	return nil
}
