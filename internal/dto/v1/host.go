package v1

// DiskUsageDTO is the per-mount disk utilisation entry shared by host snapshots
// and metric samples.
// @name DiskUsageDTO
type DiskUsageDTO struct {
	Mount   string  `json:"mount"`
	UsedPct float64 `json:"used_pct"`
}

// HostResponse is the v1 API representation of a monitored host, including its
// denormalized latest snapshot and derived online state.
// @name HostResponse
type HostResponse struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	OS           *string        `json:"os"`
	AgentVersion *string        `json:"agent_version"`
	LastSeenAt   *string        `json:"last_seen_at"`
	Online       bool           `json:"online"`
	LastCPUPct   *float64       `json:"last_cpu_pct"`
	LastMemPct   *float64       `json:"last_mem_pct"`
	LastDiskPct  *float64       `json:"last_disk_pct"`
	LastNetIn    *int64         `json:"last_net_in"`
	LastNetOut   *int64         `json:"last_net_out"`
	LastDisks    []DiskUsageDTO `json:"last_disks"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
}

// HostMetricSampleResponse is a single point-in-time host metric sample.
// @name HostMetricSampleResponse
type HostMetricSampleResponse struct {
	SampledAt string         `json:"sampled_at"`
	CPUPct    float64        `json:"cpu_pct"`
	MemPct    float64        `json:"mem_pct"`
	NetIn     int64          `json:"net_in"`
	NetOut    int64          `json:"net_out"`
	Disks     []DiskUsageDTO `json:"disks"`
}

// RegisterHostRequest is the request body for POST /api/v1/hosts.
// @name RegisterHostRequest
type RegisterHostRequest struct {
	Name string `json:"name"`
}

// CreateHostCredentialResponse carries a freshly-issued raw credential. The raw
// token is shown exactly once (registration or rotation).
// @name CreateHostCredentialResponse
type CreateHostCredentialResponse struct {
	Credential string `json:"credential"`
	Prefix     string `json:"prefix"`
}

// RegisterHostResponse is returned by POST /api/v1/hosts — the created host plus
// its one-time raw credential.
// @name RegisterHostResponse
type RegisterHostResponse struct {
	Host       HostResponse `json:"host"`
	Credential string       `json:"credential"`
	Prefix     string       `json:"prefix"`
}

// LinkHostRequest is the request body for POST /api/v1/monitors/{id}/host.
// @name LinkHostRequest
type LinkHostRequest struct {
	HostID string `json:"host_id"`
}
