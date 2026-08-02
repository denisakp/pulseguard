package v1

// Reports DTOs. camelCase, mirrors the frozen frontend
// `MonthlyReport` / `ReportHistoryEntry` / `ReportResourceBreakdown` shapes.

// ReportSettingsResponse is the monthly-report configuration (MonthlyReport).
// @name ReportSettingsResponse
type ReportSettingsResponse struct {
	Enabled        bool    `json:"enabled"`
	RecipientEmail string  `json:"recipientEmail"`
	Schedule       string  `json:"schedule"`
	Scope          string  `json:"scope"`
	LastSentAt     *string `json:"lastSentAt"`
}

// UpdateReportSettingsRequest is the body of PUT /api/v1/reports/settings.
// Mirrors MonthlyReport (lastSentAt is server-managed and ignored if sent).
// @name UpdateReportSettingsRequest
type UpdateReportSettingsRequest struct {
	Enabled        bool   `json:"enabled"`
	RecipientEmail string `json:"recipientEmail"`
	Schedule       string `json:"schedule"`
	Scope          string `json:"scope"`
}

// ReportBreakdown is one per-resource line (ReportResourceBreakdown).
// @name ReportBreakdown
type ReportBreakdown struct {
	Name      string  `json:"name"`
	UptimePct float64 `json:"uptimePct"`
	Incidents int     `json:"incidents"`
}

// ReportHistoryResponse is a generated report (ReportHistoryEntry).
// @name ReportHistoryResponse
type ReportHistoryResponse struct {
	ID                string            `json:"id"`
	Period            string            `json:"period"`
	SentAt            string            `json:"sentAt"`
	Status            string            `json:"status"`
	UptimePct         float64           `json:"uptimePct"`
	IncidentCount     int               `json:"incidentCount"`
	DowntimeSeconds   int64             `json:"downtimeSeconds"`
	RecipientEmail    string            `json:"recipientEmail"`
	ResourceBreakdown []ReportBreakdown `json:"resourceBreakdown"`
}
