package v1

import "encoding/json"

// CreateChannelRequest is the request body for POST /api/v1/notification-channels.
// @name CreateChannelRequest
type CreateChannelRequest struct {
	Name             string          `json:"name"`
	Type             string          `json:"type"`
	Config           json.RawMessage `json:"config"`
	EnabledByDefault bool            `json:"enabled_by_default"`
}

// UpdateChannelRequest is the request body for PUT/PATCH /api/v1/notification-channels/:id.
// All fields are optional (partial update). A config that omits a secret field
// (password/token/…) preserves the stored secret — see the handler's secret merge.
// @name UpdateChannelRequest
type UpdateChannelRequest struct {
	Name             *string         `json:"name,omitempty"`
	Type             *string         `json:"type,omitempty"`
	Config           json.RawMessage `json:"config,omitempty"`
	EnabledByDefault *bool           `json:"enabled_by_default,omitempty"`
}

// TestChannelConfigRequest is the request body for POST /api/v1/notification-channels/test-config
// (validate + test an unsaved channel config).
// @name TestChannelConfigRequest
type TestChannelConfigRequest struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// MessageResponse is a simple {message} success payload (test flows).
// @name MessageResponse
type MessageResponse struct {
	Message string `json:"message"`
}

// NotificationStatsResponse holds the notification counters for the admin header.
// @name NotificationStatsResponse
type NotificationStatsResponse struct {
	Sent30d   int `json:"sent_30d"`
	Pending   int `json:"pending"`
	Failed24h int `json:"failed_24h"`
}
