package email

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderReportNotification_Approved(t *testing.T) {
	message, err := RenderReportNotification(ReportNotificationData{
		ReportMonth: "April",
		ReportYear:  2026,
		EventType:   "approved",
	})
	require.NoError(t, err)

	assert.Equal(t, "Your report has been approved", message.Subject)
	assert.Contains(t, message.HTML, "Your report has been approved")
	assert.Contains(t, message.HTML, "April 2026")
	assert.Contains(t, message.HTML, "#ffffff")
	assert.Contains(t, message.HTML, "#171717")
	assert.Contains(t, message.HTML, "#3ecf8e")
	assert.Contains(t, message.HTML, "600px")
	assert.NotContains(t, message.HTML, "gradient")
	assert.Contains(t, message.PlainText, "Your report has been approved")
	assert.Contains(t, message.PlainText, "April 2026")
	assert.Contains(t, message.PlainText, "This is an automated message from AmgReports.")
}

func TestRenderReportNotification_NeedsRevision(t *testing.T) {
	message, err := RenderReportNotification(ReportNotificationData{
		ReportMonth: "April",
		ReportYear:  2026,
		EventType:   "needs_revision",
		AdminRemark: "Please clarify the weekly totals.",
	})
	require.NoError(t, err)

	assert.Equal(t, "Your report needs revision", message.Subject)
	assert.Contains(t, message.HTML, "Your report needs revision")
	assert.Contains(t, message.HTML, "April 2026")
	assert.Contains(t, message.HTML, "Please clarify the weekly totals.")
	assert.Contains(t, message.HTML, "border")
	assert.Contains(t, message.PlainText, "Your report needs revision")
	assert.Contains(t, message.PlainText, "Please clarify the weekly totals.")
}

func TestRenderReportNotification_EscapesTemplateData(t *testing.T) {
	message, err := RenderReportNotification(ReportNotificationData{
		ReportMonth: "April",
		ReportYear:  2026,
		EventType:   "needs_revision",
		AdminRemark: `<script>alert("x")</script>`,
	})
	require.NoError(t, err)

	assert.Contains(t, message.HTML, "&lt;script&gt;")
	assert.NotContains(t, message.HTML, "<script>")
	assert.Contains(t, message.PlainText, `<script>alert("x")</script>`)
}

func TestRenderReportNotification_EscapesQuotes(t *testing.T) {
	message, err := RenderReportNotification(ReportNotificationData{
		ReportMonth: "April",
		ReportYear:  2026,
		EventType:   "needs_revision",
		AdminRemark: `"Approved" <script>`,
	})
	require.NoError(t, err)

	assert.Contains(t, message.HTML, "&#34;Approved&#34;")
	assert.Contains(t, message.HTML, "&lt;script&gt;")
	assert.NotContains(t, message.HTML, `"Approved" <script>`)
	assert.Contains(t, message.PlainText, `"Approved" <script>`)
}

func TestRenderReportNotification_RejectsUnknownEvent(t *testing.T) {
	_, err := RenderReportNotification(ReportNotificationData{EventType: "unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported report notification event")
}

func TestFakeMailer_RecordsMessages(t *testing.T) {
	fake := &FakeMailer{}
	message := Message{To: "owner@example.com", Subject: "subject"}

	require.NoError(t, fake.Send(context.Background(), message))

	require.Len(t, fake.Messages, 1)
	assert.Equal(t, message, fake.Messages[0])
}

func TestFakeMailer_ReturnsConfiguredError(t *testing.T) {
	fake := &FakeMailer{Err: assert.AnError}

	err := fake.Send(context.Background(), Message{})

	assert.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, fake.Messages)
}

func TestRenderReportNotification_NoMarkupInPlainText(t *testing.T) {
	message, err := RenderReportNotification(ReportNotificationData{
		ReportMonth: "April",
		ReportYear:  2026,
		EventType:   "approved",
	})
	require.NoError(t, err)
	assert.False(t, strings.ContainsAny(message.PlainText, "<>"))
}
