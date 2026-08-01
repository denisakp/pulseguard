package v1

// MonitorResponse is the v1 API representation of a monitor (Resource).
// @name MonitorResponse
type MonitorResponse struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Type          string      `json:"type"`
	Target        string      `json:"target"`
	Interval      int         `json:"interval"`
	Timeout       int         `json:"timeout"`
	Status        string      `json:"status"`
	LastCheckedAt interface{} `json:"last_checked_at"`
	ComponentID   *string     `json:"component_id"`
	HostID        *string     `json:"host_id"` // spec 079: optional link to a monitored host (null when unlinked)
	Tags          []string    `json:"tags"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
}

// CreateMonitorRequest is the request body for POST /api/v1/monitors.
// @name CreateMonitorRequest
type CreateMonitorRequest struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Target       string   `json:"target"`
	Interval     int      `json:"interval"`
	Timeout      int      `json:"timeout"`
	ComponentID  *string  `json:"component_id,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Keyword      *string  `json:"keyword,omitempty"`
	ProtocolType *string  `json:"protocol_type,omitempty"`
	ProtocolPort *int     `json:"protocol_port,omitempty"`
}

// UpdateMonitorRequest is the request body for PUT /api/v1/monitors/:id.
// All fields are optional (PATCH semantics).
// @name UpdateMonitorRequest
type UpdateMonitorRequest struct {
	Name         *string  `json:"name,omitempty"`
	Type         *string  `json:"type,omitempty"`
	Target       *string  `json:"target,omitempty"`
	Interval     *int     `json:"interval,omitempty"`
	Timeout      *int     `json:"timeout,omitempty"`
	ComponentID  *string  `json:"component_id,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Keyword      *string  `json:"keyword,omitempty"`
	ProtocolType *string  `json:"protocol_type,omitempty"`
	ProtocolPort *int     `json:"protocol_port,omitempty"`
}

// HeartbeatPingResponse is the v1 API response for a successful heartbeat ping.
// @name HeartbeatPingResponse
type HeartbeatPingResponse struct {
	ReceivedAt string `json:"received_at"`
}
