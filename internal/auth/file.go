package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// File is the on-disk Audible device-registration bundle.
// Field names match the existing audible-auth.json schema so bundles
// created by the original TypeScript CLI keep working.
type File struct {
	AccessToken               string            `json:"accessToken"`
	AdpToken                  string            `json:"adpToken"`
	CustomerInfo              map[string]any    `json:"customerInfo,omitempty"`
	DeviceInfo                map[string]any    `json:"deviceInfo,omitempty"`
	DevicePrivateKey          string            `json:"devicePrivateKey"`
	Domain                    string            `json:"domain"`
	ExpiresAt                 int64             `json:"expiresAt"`
	Locale                    string            `json:"locale"`
	RefreshToken              string            `json:"refreshToken"`
	Serial                    string            `json:"serial"`
	StoreAuthenticationCookie map[string]any    `json:"storeAuthenticationCookie,omitempty"`
	WebsiteCookies            map[string]string `json:"websiteCookies,omitempty"`
	WithUsername              bool              `json:"withUsername"`
}

// Load reads and validates an auth bundle from path.
func Load(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read auth file: %w", err)
	}
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse auth file: %w", err)
	}
	if err := f.validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

// Save writes the auth bundle as pretty JSON, creating parent directories.
func Save(path string, f *File) error {
	if err := f.validate(); err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return fmt.Errorf("create auth file directory: %w", err)
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(abs, raw, 0o600); err != nil {
		return fmt.Errorf("write auth file: %w", err)
	}
	return nil
}

func (f *File) validate() error {
	if f == nil {
		return fmt.Errorf("auth file is nil")
	}
	missing := make([]string, 0, 6)
	if strings.TrimSpace(f.AccessToken) == "" {
		missing = append(missing, "accessToken")
	}
	if strings.TrimSpace(f.AdpToken) == "" {
		missing = append(missing, "adpToken")
	}
	if strings.TrimSpace(f.DevicePrivateKey) == "" {
		missing = append(missing, "devicePrivateKey")
	}
	if strings.TrimSpace(f.Domain) == "" {
		missing = append(missing, "domain")
	}
	if strings.TrimSpace(f.RefreshToken) == "" {
		missing = append(missing, "refreshToken")
	}
	if strings.TrimSpace(f.Serial) == "" {
		missing = append(missing, "serial")
	}
	if len(missing) > 0 {
		return fmt.Errorf("auth file missing required fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (f *File) apiHost() string {
	if f.WithUsername {
		return "audible." + f.Domain
	}
	return "amazon." + f.Domain
}
