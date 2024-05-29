package userv1

import (
	"encoding/json"
	"fmt"
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// MarshalJSON custom implementation for User struct
func (p *User) MarshalJSON() ([]byte, error) {
	type Alias User // Create an alias to prevent recursion
	return json.Marshal(&struct {
		CreatedAt string `json:"created_at,omitempty"`
		*Alias
	}{
		CreatedAt: p.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z07:00"),
		Alias:     (*Alias)(p),
	})
}

// UnmarshalJSON custom implementation for User struct
func (p *User) UnmarshalJSON(data []byte) error {
	type Alias User
	aux := &struct {
		CreatedAt string `json:"created_at,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.CreatedAt != "" {
		t, err := time.Parse("2006-01-02T15:04:05Z07:00", aux.CreatedAt)
		if err != nil {
			return fmt.Errorf("invalid time format: %w", err)
		}
		p.CreatedAt = timestamppb.New(t)
	}
	return nil
}
