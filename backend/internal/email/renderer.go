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

//go:embed templates/report_notification.html.tmpl
var htmlTemplates embed.FS

//go:embed templates/report_notification.txt.tmpl
var textTemplates embed.FS

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
