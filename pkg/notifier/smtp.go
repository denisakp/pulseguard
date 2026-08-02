package notifier

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"strconv"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	gomail "gopkg.in/mail.v2"
)

//go:embed templates/*.html
var emailTemplates embed.FS

const (
	errParseEmailTemplate   = "failed to parse email template"
	errExecuteEmailTemplate = "failed to execute email template"
)

// SMTPNotifier implements email notifications using SMTP.
type SMTPNotifier struct {
	recipient    string
	sender       string
	smtpHost     string
	smtpPort     string
	smtpUser     string
	smtpPassword string
}

type TemplateData struct {
	Incident     domain.Incident
	Duration     string
	ErrorSummary string
	Keyword      string
	KeywordMode  string
}

type ComponentTemplateData struct {
	Name     string
	Status   domain.ComponentStatus
	Impacted []ComponentResource
}

type ExpiryTemplateData struct {
	ResourceName  string
	ResourceURL   string
	ExpiryType    string
	DaysRemaining int
	ExpiresAt     string
	Issuer        string
	Threshold     int
}

type TestTemplateData struct {
	Message   string
	Timestamp string
}

// NewSMTPNotifier creates a new SMTP notifier instance with the provided configuration.
func NewSMTPNotifier(recipient, sender, smtpHost, smtpPort, smtpUser, smtpPassword string) *SMTPNotifier {
	return &SMTPNotifier{
		recipient:    recipient,
		sender:       sender,
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUser:     smtpUser,
		smtpPassword: smtpPassword,
	}
}

func (n *SMTPNotifier) validateConfig() error {
	if n.recipient == "" {
		return fmt.Errorf("smtp recipient is not configured")
	}
	if n.sender == "" {
		return fmt.Errorf("smtp sender is not configured")
	}
	if n.smtpHost == "" {
		return fmt.Errorf("smtp host is not configured")
	}
	if n.smtpPort == "" {
		return fmt.Errorf("smtp port is not configured")
	}
	if n.smtpUser == "" {
		return fmt.Errorf("smtp user is not configured")
	}
	if n.smtpPassword == "" {
		return fmt.Errorf("smtp password is not configured")
	}
	return nil
}

// Send sends an email notification for either a resource incident or a component state change.
func (n *SMTPNotifier) Send(ctx context.Context, payload NotificationPayload) error {
	if err := n.validateConfig(); err != nil {
		return err
	}

	smtpPort, err := strconv.Atoi(n.smtpPort)
	if err != nil {
		return fmt.Errorf("invalid smtp_port: %w", err)
	}

	var subject string
	var htmlBody string

	switch {
	case payload.Flapping != nil:
		subject = n.flappingSubject(payload.Flapping)
		htmlBody = n.generateFlappingEmailHTML(payload.Flapping)
	case payload.Reminder != nil:
		subject = n.reminderSubject(payload.Reminder)
		htmlBody = n.generateReminderEmailHTML(payload.Reminder)
	case payload.Component != nil:
		subject = n.componentSubject(payload.Component)
		htmlBody = n.generateComponentEmailHTML(payload.Component)
	case payload.Expiry != nil:
		subject = n.expirySubject(payload.Expiry)
		htmlBody = n.generateExpiryEmailHTML(payload.Expiry)
	case payload.Report != nil:
		subject = n.reportSubject(payload.Report)
		htmlBody = n.generateReportEmailHTML(payload.Report)
	case payload.Operator != nil:
		subject = payload.Operator.Title
		htmlBody = n.generateOperatorEmailHTML(payload.Operator)
	case payload.Incident != nil:
		incident := *payload.Incident
		subject, htmlBody = n.incidentEmailContent(incident)
	default:
		return fmt.Errorf("notification payload missing incident, component, expiry, flapping, reminder, report, or operator")
	}

	message := gomail.NewMessage()
	message.SetHeader("From", n.sender)
	message.SetHeader("To", n.recipient)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", htmlBody)

	dialer := gomail.NewDialer(n.smtpHost, smtpPort, n.smtpUser, n.smtpPassword)

	if err := dialer.DialAndSend(message); err != nil {
		slog.Error("failed to send email", "recipient", n.recipient, "error", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	slog.Info("email sent successfully", "recipient", n.recipient, "subject", subject)
	return nil
}

func (n *SMTPNotifier) incidentEmailContent(incident domain.Incident) (string, string) {
	isResolved := incident.ResolvedAt != nil

	if isResolved {
		subject := fmt.Sprintf("✅ RESOLVED: %s is back online", incident.Resource.Name)
		return subject, n.generateUpEmailHTML(incident)
	}

	subject := fmt.Sprintf("🔴 ALERT: %s is down", incident.Resource.Name)
	return subject, n.generateDownEmailHTML(incident)
}

func (n *SMTPNotifier) componentSubject(component *ComponentNotification) string {
	switch component.Status {
	case domain.ComponentStatusDown:
		return fmt.Sprintf("🔴 Component %s is down", component.Component.Name)
	case domain.ComponentStatusDegraded:
		return fmt.Sprintf("⚠️ Component %s degraded", component.Component.Name)
	default:
		return fmt.Sprintf("✅ Component %s recovered", component.Component.Name)
	}
}

// generateDownEmailHTML creates an HTML email for resource down events.
func (n *SMTPNotifier) generateDownEmailHTML(incident domain.Incident) string {
	data := &TemplateData{Incident: incident}
	if incident.Resource.Type == domain.ResourceKeyword {
		if incident.Resource.Keyword != nil {
			data.Keyword = *incident.Resource.Keyword
		}
		if incident.Resource.KeywordMode != nil {
			data.KeywordMode = *incident.Resource.KeywordMode
		}
		data.ErrorSummary = domain.GenerateErrorSummary(incident.Cause, -1)
	}

	tmpl, err := template.ParseFS(emailTemplates, "templates/resource_down.html")
	if err != nil {
		slog.Error(errParseEmailTemplate, "template", "resource_down.html", "error", err)
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>Resource %s is down.</p></body></html>", incident.Resource.Name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error(errExecuteEmailTemplate, "template", "resource_down.html", "resource", incident.Resource.Name, "incident_id", incident.ID, "error", err)
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>Resource %s is down. Incident ID: %s</p></body></html>", incident.Resource.Name, incident.ID)
	}

	return buf.String()
}

// generateUpEmailHTML creates an HTML email for resource recovery events.
func (n *SMTPNotifier) generateUpEmailHTML(incident domain.Incident) string {
	duration := "N/A"
	if incident.ResolvedAt != nil {
		d := incident.ResolvedAt.Sub(incident.StartedAt)
		duration = formatDuration(d)
	}

	data := TemplateData{Incident: incident, Duration: duration}

	tmpl, err := template.ParseFS(emailTemplates, "templates/resource_up.html")
	if err != nil {
		slog.Error(errParseEmailTemplate, "template", "resource_up.html", "error", err)
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>Resource %s is back online.</p></body></html>", incident.Resource.Name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error(errExecuteEmailTemplate, "template", "resource_up.html", "resource", incident.Resource.Name, "incident_id", incident.ID, "error", err)
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>Resource %s is back online. Downtime: %s</p></body></html>", incident.Resource.Name, duration)
	}

	return buf.String()
}

// generateComponentEmailHTML creates an HTML email for component state changes.
func (n *SMTPNotifier) generateComponentEmailHTML(component *ComponentNotification) string {
	data := &ComponentTemplateData{
		Name:     component.Component.Name,
		Status:   component.Status,
		Impacted: component.Impacted,
	}

	tmpl, err := template.ParseFS(emailTemplates, "templates/component_status.html")
	if err != nil {
		slog.Error(errParseEmailTemplate, "template", "component_status.html", "error", err)
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>Component %s is %s.</p></body></html>", component.Component.Name, component.Status)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error(errExecuteEmailTemplate, "template", "component_status.html", "component", component.Component.Name, "error", err)
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>Component %s is %s.</p></body></html>", component.Component.Name, component.Status)
	}

	return buf.String()
}

// generateTestEmailHTML creates an HTML email for test notifications.
func (n *SMTPNotifier) generateTestEmailHTML(resource domain.Resource) string {
	data := TestTemplateData{
		Message:   fmt.Sprintf("Test notification for resource: %s (%s)", resource.Name, resource.Target),
		Timestamp: time.Now().Format("2006-01-02 15:04:05 MST"),
	}

	tmpl, err := template.ParseFS(emailTemplates, "templates/test.html")
	if err != nil {
		slog.Error(errParseEmailTemplate, "template", "test.html", "error", err)
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>Test notification for resource %s.</p></body></html>", resource.Name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error(errExecuteEmailTemplate, "template", "test.html", "error", err)
		return fmt.Sprintf("error generating test email content for resource %s", resource.Name)
	}

	return buf.String()
}

// formatDuration formats a duration into a human-readable string.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// SendTestNotification sends a test email notification for a resource.
// It uses the test.html template to send a simple test message.
func (n *SMTPNotifier) SendTestNotification(ctx context.Context, resource domain.Resource) error {
	// Validate configuration
	if n.recipient == "" {
		return fmt.Errorf("smtp recipient is not configured")
	}
	if n.sender == "" {
		return fmt.Errorf("smtp sender is not configured")
	}
	if n.smtpHost == "" {
		return fmt.Errorf("smtp host is not configured")
	}
	if n.smtpPort == "" {
		return fmt.Errorf("smtp port is not configured")
	}
	if n.smtpUser == "" {
		return fmt.Errorf("smtp user is not configured")
	}
	if n.smtpPassword == "" {
		return fmt.Errorf("smtp password is not configured")
	}

	smtpPort, err := strconv.Atoi(n.smtpPort)
	if err != nil {
		return fmt.Errorf("invalid smtp_port: %w", err)
	}

	// Generate test email content
	subject := fmt.Sprintf("ℹ️ Test Notification - %s", resource.Name)
	htmlBody := n.generateTestEmailHTML(resource)

	// Create email message
	message := gomail.NewMessage()
	message.SetHeader("From", n.sender)
	message.SetHeader("To", n.recipient)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", htmlBody)

	// Create SMTP dialer
	dialer := gomail.NewDialer(n.smtpHost, smtpPort, n.smtpUser, n.smtpPassword)

	// Send the email
	if err := dialer.DialAndSend(message); err != nil {
		slog.Error("failed to send test email", "recipient", n.recipient, "error", err)
		return fmt.Errorf("failed to send test email: %w", err)
	}

	slog.Info("test email sent successfully", "recipient", n.recipient, "subject", subject)
	return nil
}

func (n *SMTPNotifier) expirySubject(expiry *ExpiryNotification) string {
	typeLabel := "SSL"
	if expiry.ExpiryType == "domain" {
		typeLabel = "Domain"
	}
	return fmt.Sprintf("%s expiry alert: %s expires in %d days", typeLabel, expiry.Resource.Name, expiry.DaysRemaining)
}

func (n *SMTPNotifier) flappingSubject(flapping *FlappingNotification) string {
	if flapping.Stabilized {
		return fmt.Sprintf("[Ogoune] %s stabilized after flapping", flapping.Resource.Name)
	}
	return fmt.Sprintf("[Ogoune] %s is flapping - %d transitions in %d minutes", flapping.Resource.Name, flapping.TransitionCount, flapping.WindowSeconds/60)
}

func (n *SMTPNotifier) generateFlappingEmailHTML(flapping *FlappingNotification) string {
	if flapping.Stabilized {
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>Monitor <strong>%s</strong> stabilized after a flapping episode.</p><p>Current status: %s</p></body></html>", flapping.Resource.Name, flapping.FinalStatus)
	}
	return fmt.Sprintf("<!DOCTYPE html><html><body><p>Your monitor <strong>%s</strong> (%s) has been switching between UP and DOWN %d times in the last %d minutes.</p><p>Ogoune has suppressed repeated alerts while the service is unstable.</p></body></html>", flapping.Resource.Name, flapping.Resource.Target, flapping.TransitionCount, flapping.WindowSeconds/60)
}

func (n *SMTPNotifier) reminderSubject(reminder *ReminderNotification) string {
	return fmt.Sprintf("[Ogoune] %s still down - %d minutes", reminder.Resource.Name, reminder.ElapsedMinutes)
}

func (n *SMTPNotifier) generateReminderEmailHTML(reminder *ReminderNotification) string {
	return fmt.Sprintf("<!DOCTYPE html><html><body><p>Your monitor <strong>%s</strong> has been down for %d minutes.</p><p>Incident started at %s.</p><p>This is a reminder - the incident is still active.</p></body></html>", reminder.Resource.Name, reminder.ElapsedMinutes, reminder.Incident.StartedAt.Format(time.RFC3339))
}

func (n *SMTPNotifier) generateExpiryEmailHTML(expiry *ExpiryNotification) string {
	data := &ExpiryTemplateData{
		ResourceName:  expiry.Resource.Name,
		ResourceURL:   expiry.Resource.Target,
		ExpiryType:    expiry.ExpiryType,
		DaysRemaining: expiry.DaysRemaining,
		ExpiresAt:     expiry.ExpiresAt.Format("2006-01-02"),
		Issuer:        expiry.Issuer,
		Threshold:     expiry.Threshold,
	}

	tmpl, err := template.ParseFS(emailTemplates, "templates/expiry_alert.html")
	if err != nil {
		slog.Error(errParseEmailTemplate, "template", "expiry_alert.html", "error", err)
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>%s expiry alert: %s expires in %d days.</p></body></html>",
			expiry.ExpiryType, expiry.Resource.Name, expiry.DaysRemaining)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error(errExecuteEmailTemplate, "template", "expiry_alert.html", "resource", expiry.Resource.Name, "error", err)
		return fmt.Sprintf("<!DOCTYPE html><html><body><p>%s expiry alert: %s expires in %d days.</p></body></html>",
			expiry.ExpiryType, expiry.Resource.Name, expiry.DaysRemaining)
	}

	return buf.String()
}

// generateOperatorEmailHTML renders a generic operator message (agent-down alert
// or unread digest). Inline HTML (no embedded template), like the report email.
func (n *SMTPNotifier) generateOperatorEmailHTML(op *OperatorNotification) string {
	list := ""
	if len(op.Items) > 0 {
		var items bytes.Buffer
		for _, it := range op.Items {
			items.WriteString(fmt.Sprintf("<li style=\"margin:2px 0\">%s</li>", template.HTMLEscapeString(it)))
		}
		list = fmt.Sprintf("<ul style=\"margin:12px 0 0;padding-left:20px;color:#334155\">%s</ul>", items.String())
	}
	return fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family:system-ui,sans-serif;color:#0f172a">
<h2 style="margin:0 0 8px">%s</h2>
<p style="margin:0;color:#334155">%s</p>%s
</body></html>`, template.HTMLEscapeString(op.Title), template.HTMLEscapeString(op.Body), list)
}

// reportSubject builds the subject line for a monthly report email.
func (n *SMTPNotifier) reportSubject(r *ReportNotification) string {
	return fmt.Sprintf("Monthly uptime report — %s", r.Period)
}

// generateReportEmailHTML renders a simple, dependency-free HTML body for a
// monthly report. Kept inline (not templated) to avoid a new embedded asset.
func (n *SMTPNotifier) generateReportEmailHTML(r *ReportNotification) string {
	var rows bytes.Buffer
	for _, b := range r.Breakdown {
		rows.WriteString(fmt.Sprintf(
			"<tr><td style=\"padding:4px 12px 4px 0\">%s</td><td style=\"padding:4px 12px;text-align:right\">%.2f%%</td><td style=\"padding:4px 0;text-align:right\">%d</td></tr>",
			template.HTMLEscapeString(b.Name), b.UptimePct, b.Incidents))
	}
	downtimeMin := r.DowntimeSeconds / 60
	return fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family:system-ui,sans-serif;color:#0f172a">
<h2 style="margin:0 0 4px">Monthly uptime report</h2>
<p style="color:#64748b;margin:0 0 16px">Period: %s</p>
<table style="border-collapse:collapse;margin-bottom:16px">
<tr><td style="padding:4px 24px 4px 0"><strong>Overall uptime</strong></td><td style="text-align:right">%.2f%%</td></tr>
<tr><td style="padding:4px 24px 4px 0"><strong>Incidents</strong></td><td style="text-align:right">%d</td></tr>
<tr><td style="padding:4px 24px 4px 0"><strong>Total downtime</strong></td><td style="text-align:right">%d min</td></tr>
</table>
<h3 style="margin:0 0 8px">Per-resource</h3>
<table style="border-collapse:collapse"><thead><tr><th style="text-align:left;padding:0 12px 4px 0">Resource</th><th style="text-align:right;padding:0 12px 4px">Uptime</th><th style="text-align:right;padding:0 0 4px">Incidents</th></tr></thead><tbody>%s</tbody></table>
</body></html>`, template.HTMLEscapeString(r.Period), r.UptimePct, r.IncidentCount, downtimeMin, rows.String())
}
