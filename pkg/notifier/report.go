package notifier

// ReportBreakdownLine is one per-resource line in a monthly report email.
type ReportBreakdownLine struct {
	Name      string
	UptimePct float64
	Incidents int
}

// ReportNotification carries a generated monthly report for email dispatch.
type ReportNotification struct {
	Period          string // YYYY-MM
	Recipient       string
	UptimePct       float64
	IncidentCount   int
	DowntimeSeconds int64
	Breakdown       []ReportBreakdownLine
}
