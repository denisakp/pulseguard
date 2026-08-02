package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
)

// WebHookNotifier is a notifier that sends notifications via webhook.
type WebHookNotifier struct {
	client *http.Client
	url    string
	secret *string
}

// NewWebHookNotifier creates a new WebHookNotifier with the provided URL and optional secret.
func NewWebHookNotifier(url string, secret *string) *WebHookNotifier {
	return &WebHookNotifier{
		url:    url,
		secret: secret,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

// Send sends a notification via webhook.
func (n *WebHookNotifier) Send(ctx context.Context, payload NotificationPayload) error {
	if n.url == "" {
		return fmt.Errorf("webhook url is empty")
	}

	var body map[string]any

	switch {
	case payload.Flapping != nil:
		flapping := payload.Flapping
		event := "flapping"
		body = map[string]any{
			"event":            event,
			"resource_id":      flapping.Resource.ID,
			"resource_name":    flapping.Resource.Name,
			"target":           flapping.Resource.Target,
			"transition_count": flapping.TransitionCount,
			"window_seconds":   flapping.WindowSeconds,
			"triggered_at":     flapping.TriggeredAt.Format(time.RFC3339),
		}
		if flapping.Stabilized {
			body["event"] = "flapping_stabilized"
			body["final_status"] = flapping.FinalStatus
		}
	case payload.Reminder != nil:
		reminder := payload.Reminder
		body = map[string]any{
			"event":               "reminder",
			"resource_id":         reminder.Resource.ID,
			"resource_name":       reminder.Resource.Name,
			"target":              reminder.Resource.Target,
			"incident_id":         reminder.Incident.ID,
			"incident_started_at": reminder.Incident.StartedAt.Format(time.RFC3339),
			"elapsed_minutes":     reminder.ElapsedMinutes,
			"triggered_at":        reminder.TriggeredAt.Format(time.RFC3339),
		}
	case payload.Component != nil:
		component := payload.Component
		impacted := make([]map[string]string, 0, len(component.Impacted))
		for _, r := range component.Impacted {
			impacted = append(impacted, map[string]string{
				"id":     r.ID,
				"name":   r.Name,
				"status": string(r.Status),
			})
		}

		body = map[string]any{
			"type":      "component",
			"component": component.Component.Name,
			"status":    component.Status,
			"impacted":  impacted,
		}
	case payload.Expiry != nil:
		expiry := payload.Expiry
		body = map[string]any{
			"type":           "expiry",
			"event_type":     "expiry",
			"resource_id":    expiry.Resource.ID,
			"resource_name":  expiry.Resource.Name,
			"resource_url":   expiry.Resource.Target,
			"expiry_type":    expiry.ExpiryType,
			"days_remaining": expiry.DaysRemaining,
			"expires_at":     expiry.ExpiresAt.Format(time.RFC3339),
			"issuer":         expiry.Issuer,
			"threshold":      expiry.Threshold,
			"triggered_at":   expiry.TriggeredAt.Format(time.RFC3339),
		}
	case payload.Incident != nil:
		incident := payload.Incident
		status := "DOWN"
		if incident.ResolvedAt != nil {
			status = "UP"
		}
		body = map[string]any{
			"type":    "incident",
			"status":  status,
			"message": incident.Cause,
		}
		if incident.Resource.Type == domain.ResourceKeyword {
			if incident.Resource.Keyword != nil {
				body["keyword"] = *incident.Resource.Keyword
			}
			if incident.Resource.KeywordMode != nil {
				body["keyword_mode"] = *incident.Resource.KeywordMode
			}
			body["failure_cause"] = incident.Cause
		}
	case payload.Operator != nil:
		op := payload.Operator
		body = map[string]any{
			"type":  "operator",
			"title": op.Title,
			"body":  op.Body,
			"items": op.Items,
		}
	default:
		return fmt.Errorf("notification payload missing incident, component, expiry, flapping, reminder, or operator")
	}

	payloadBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// create http request
	req, err := http.NewRequestWithContext(ctx, "POST", n.url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// set the header signature if exists.
	if n.secret != nil && *n.secret != "" {
		// generate HMac signature for the payload
		mac := hmac.New(sha256.New, []byte(*n.secret))
		mac.Write(payloadBytes)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		req.Header.Set("X-Ogoune-Signature", signature)
	}

	response, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook notification: %w", err)
	}
	defer response.Body.Close()

	// check the response status
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-success status: %d", response.StatusCode)
	}

	return nil
}

func (n *WebHookNotifier) SendTestNotification(ctx context.Context) error {
	if n.url == "" {
		return fmt.Errorf("webhook url is empty")
	}

	payloadBytes, err := json.Marshal(map[string]interface{}{
		"status":  "TEST",
		"message": "Test notification",
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// create http request
	req, err := http.NewRequestWithContext(ctx, "POST", n.url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// set the header signature if exists.
	if n.secret != nil && *n.secret != "" {
		// generate HMac signature for the payload
		mac := hmac.New(sha256.New, []byte(*n.secret))
		mac.Write(payloadBytes)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		req.Header.Set("X-Ogoune-Signature", signature)
	}

	response, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook notification: %w", err)
	}
	defer response.Body.Close()

	// check the response status
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-success status: %d", response.StatusCode)
	}

	return nil
}
