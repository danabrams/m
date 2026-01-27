package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Platform represents a device platform.
type Platform string

const (
	PlatformIOS Platform = "ios"
)

// Device represents a registered device for push notifications.
type Device struct {
	Token     string
	Platform  Platform
	CreatedAt time.Time
}

// CreateDevice registers a new device token.
func (s *Store) CreateDevice(token string, platform Platform) (*Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()

	// Use INSERT OR REPLACE to handle re-registration
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO devices (token, platform, created_at)
		 VALUES (?, ?, ?)`,
		token, string(platform), now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert device: %w", err)
	}

	return &Device{
		Token:     token,
		Platform:  platform,
		CreatedAt: time.Unix(now, 0),
	}, nil
}

// GetDevice retrieves a device by token.
func (s *Store) GetDevice(token string) (*Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var device Device
	var platform string
	var createdAt int64

	err := s.db.QueryRow(
		"SELECT token, platform, created_at FROM devices WHERE token = ?",
		token,
	).Scan(&device.Token, &platform, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query device: %w", err)
	}

	device.Platform = Platform(platform)
	device.CreatedAt = time.Unix(createdAt, 0)
	return &device, nil
}

// ListDevices retrieves all registered devices.
func (s *Store) ListDevices() ([]*Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT token, platform, created_at FROM devices ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		var platform string
		var createdAt int64
		if err := rows.Scan(&device.Token, &platform, &createdAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		device.Platform = Platform(platform)
		device.CreatedAt = time.Unix(createdAt, 0)
		devices = append(devices, &device)
	}

	return devices, rows.Err()
}

// ListDevicesByPlatform retrieves devices for a specific platform.
func (s *Store) ListDevicesByPlatform(platform Platform) ([]*Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT token, platform, created_at FROM devices WHERE platform = ? ORDER BY created_at DESC",
		string(platform),
	)
	if err != nil {
		return nil, fmt.Errorf("query devices by platform: %w", err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		var p string
		var createdAt int64
		if err := rows.Scan(&device.Token, &p, &createdAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		device.Platform = Platform(p)
		device.CreatedAt = time.Unix(createdAt, 0)
		devices = append(devices, &device)
	}

	return devices, rows.Err()
}

// DeleteDevice removes a device by token.
func (s *Store) DeleteDevice(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM devices WHERE token = ?", token)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}
