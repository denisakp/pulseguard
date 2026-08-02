package v1

import "time"

// AddTagsRequest is the body for POST /api/v1/monitors/{id}/tags (spec 085).
// @name AddTagsRequest
type AddTagsRequest struct {
	TagIDs []string `json:"tag_ids"`
}

// MonitorUptimeStat is one hourly uptime bucket.
// @name MonitorUptimeStat
type MonitorUptimeStat struct {
	Hour            time.Time `json:"hour"`
	UptimePercent   float64   `json:"uptime_percent"`
	SuccessfulCount int       `json:"successful_count"`
	TotalCount      int       `json:"total_count"`
}

// MonitorUptimeStatsResponse is the payload of GET /api/v1/monitors/{id}/uptime-stats.
// @name MonitorUptimeStatsResponse
type MonitorUptimeStatsResponse struct {
	ResourceID string              `json:"resource_id"`
	Stats      []MonitorUptimeStat `json:"stats"`
}
