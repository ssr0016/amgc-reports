package email

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"
)

type ReportNotificationData struct {
	ReportMonth string
	ReportYear  int
	EventType   string
	AdminRemark string
}

type DailyDigestData struct {
	ReportMonth  string
	ReportYear   int
	PendingCount int
	MissingCount int
	ReportsURL   string
}

//go:embed templates/report_notification.html.tmpl
var htmlTemplates embed.FS

//go:embed templates/report_notification.txt.tmpl
var textTemplates embed.FS

//go:embed templates/daily_digest.html.tmpl
var dailyDigestHTMLTemplates embed.FS

//go:embed templates/daily_digest.txt.tmpl
var dailyDigestTextTemplates embed.FS

func RenderDailyDigest(data DailyDigestData) (Message, error) {
	h, err := htmltemplate.ParseFS(dailyDigestHTMLTemplates, "templates/daily_digest.html.tmpl")
	if err != nil {
		return Message{}, fmt.Errorf("parse HTML daily digest template: %w", err)
	}
	t, err := texttemplate.ParseFS(dailyDigestTextTemplates, "templates/daily_digest.txt.tmpl")
	if err != nil {
		return Message{}, fmt.Errorf("parse plain-text daily digest template: %w", err)
	}
	var htmlBody, textBody bytes.Buffer
	if err := h.Execute(&htmlBody, data); err != nil {
		return Message{}, fmt.Errorf("render HTML daily digest: %w", err)
	}
	if err := t.Execute(&textBody, data); err != nil {
		return Message{}, fmt.Errorf("render plain-text daily digest: %w", err)
	}
	return Message{Subject: "Daily report digest", HTML: htmlBody.String(), PlainText: textBody.String()}, nil
}

func RenderReportNotification(data ReportNotificationData) (Message, error) {
	subject, err := reportNotificationSubject(data.EventType)
	if err != nil {
		return Message{}, err
	}

	htmlTemplate, err := htmltemplate.ParseFS(htmlTemplates, "templates/report_notification.html.tmpl")
	if err != nil {
		return Message{}, fmt.Errorf("parse HTML report notification template: %w", err)
	}
	textTemplate, err := texttemplate.ParseFS(textTemplates, "templates/report_notification.txt.tmpl")
	if err != nil {
		return Message{}, fmt.Errorf("parse plain-text report notification template: %w", err)
	}

	var htmlBody bytes.Buffer
	if err := htmlTemplate.Execute(&htmlBody, data); err != nil {
		return Message{}, fmt.Errorf("render HTML report notification: %w", err)
	}
	var textBody bytes.Buffer
	if err := textTemplate.Execute(&textBody, data); err != nil {
		return Message{}, fmt.Errorf("render plain-text report notification: %w", err)
	}

	return Message{Subject: subject, PlainText: textBody.String(), HTML: htmlBody.String()}, nil
}

func reportNotificationSubject(eventType string) (string, error) {
	switch eventType {
	case "approved":
		return "Your report has been approved", nil
	case "needs_revision":
		return "Your report needs revision", nil
	default:
		return "", fmt.Errorf("unsupported report notification event %q", eventType)
	}
}
