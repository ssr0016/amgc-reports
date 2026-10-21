package report_test

import (
	"encoding/json"
	"math/big"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

func num(units int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(units), Exp: -2, Valid: true}
}

func ptr[T any](v T) *T { return &v }

func TestReport_ToResponse(t *testing.T) {
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	deleted := now

	t.Run("nil optionals become empty strings and deleted_at is not exposed", func(t *testing.T) {
		r := report.Report{ID: 7, CreatedBy: 3, Month: "May", Year: 2026, DeletedAt: &deleted, CreatedAt: now, UpdatedAt: now}

		resp := r.ToResponse()

		assert.Equal(t, int64(7), resp.ID)
		assert.Equal(t, 2026, resp.Year)
		assert.Empty(t, resp.Worker)
		assert.NotNil(t, resp.Metrics, "metrics must serialise as {} not null")
		assert.Empty(t, resp.Metrics)
		raw, err := json.Marshal(resp)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "deleted_at")
		assert.Contains(t, string(raw), `"metrics":{}`)
	})

	t.Run("metrics are grouped by key with absent weeks omitted", func(t *testing.T) {
		r := report.Report{
			Worker: ptr("Ana"),
			Metrics: map[string]map[int]pgtype.Numeric{
				"worship_service": {1: num(12000), 3: num(0)},
				"tithes_offering": {2: num(123450)},
			},
		}

		resp := r.ToResponse()

		assert.Equal(t, "Ana", resp.Worker)
		ws := resp.Metrics["worship_service"]
		require.NotNil(t, ws.Week1)
		assert.Equal(t, json.Number("120.00"), *ws.Week1)
		assert.Nil(t, ws.Week2, "absent is not zero")
		require.NotNil(t, ws.Week3)
		assert.Equal(t, json.Number("0.00"), *ws.Week3, "reported zero stays distinct")
		raw, err := json.Marshal(resp.Metrics)
		require.NoError(t, err)
		assert.JSONEq(t, `{"worship_service":{"week1":120.00,"week3":0.00},"tithes_offering":{"week2":1234.50}}`, string(raw))
	})
}

func validCreate() report.CreateReportRequest {
	return report.CreateReportRequest{Month: "May"}
}

func repeat(n int) string { return strings.Repeat("é", n) } // multi-byte: limits count characters

func TestCreateReportRequest_Validation(t *testing.T) {
	v := validator.New()
	n := func(s string) *json.Number { j := json.Number(s); return &j }
	metrics := func(key string, w report.MetricWeeks) map[string]report.MetricWeeks {
		return map[string]report.MetricWeeks{key: w}
	}

	tests := []struct {
		name      string
		mutate    func(*report.CreateReportRequest)
		wantField string // "" = valid
	}{
		{"minimal valid", func(*report.CreateReportRequest) {}, ""},
		{"month required", func(r *report.CreateReportRequest) { r.Month = "" }, "month"},
		{"month exact casing", func(r *report.CreateReportRequest) { r.Month = "may" }, "month"},
		{"year omitted ok", func(r *report.CreateReportRequest) { r.Year = nil }, ""},
		{"year 2000 ok", func(r *report.CreateReportRequest) { r.Year = ptr(2000) }, ""},
		{"year 2100 ok", func(r *report.CreateReportRequest) { r.Year = ptr(2100) }, ""},
		{"year 1999", func(r *report.CreateReportRequest) { r.Year = ptr(1999) }, "year"},
		{"year 2101", func(r *report.CreateReportRequest) { r.Year = ptr(2101) }, "year"},
		{"worker 150 chars ok", func(r *report.CreateReportRequest) { r.Worker = repeat(150) }, ""},
		{"worker 151 chars", func(r *report.CreateReportRequest) { r.Worker = repeat(151) }, "worker"},
		{"area 151 chars", func(r *report.CreateReportRequest) { r.AreaAssignment = repeat(151) }, "area_assignment"},
		{"church 151 chars", func(r *report.CreateReportRequest) { r.ChurchName = repeat(151) }, "church_name"},
		{"names 5000 ok", func(r *report.CreateReportRequest) { r.Names = repeat(5000) }, ""},
		{"names 5001", func(r *report.CreateReportRequest) { r.Names = repeat(5001) }, "names"},
		{"narrative 10000 ok", func(r *report.CreateReportRequest) { r.NarrativeReport = repeat(10000) }, ""},
		{"narrative 10001", func(r *report.CreateReportRequest) { r.NarrativeReport = repeat(10001) }, "narrative_report"},
		{"challenges 10001", func(r *report.CreateReportRequest) { r.Challenges = repeat(10001) }, "challenges"},
		{"prayer 10001", func(r *report.CreateReportRequest) { r.PrayerRequest = repeat(10001) }, "prayer_request"},
		{"snake_case metric ok", func(r *report.CreateReportRequest) {
			r.Metrics = metrics("worship_service", report.MetricWeeks{Week1: n("1234.50")})
		}, ""},
		{"camelCase metric rejected (D-6)", func(r *report.CreateReportRequest) {
			r.Metrics = metrics("worshipService", report.MetricWeeks{Week1: n("1")})
		}, "metrics[worshipService]"},
		{"zero value ok", func(r *report.CreateReportRequest) {
			r.Metrics = metrics("outreach", report.MetricWeeks{Week5: n("0")})
		}, ""},
		{"max value ok", func(r *report.CreateReportRequest) {
			r.Metrics = metrics("outreach", report.MetricWeeks{Week2: n("9999999999.99")})
		}, ""},
		{"negative value", func(r *report.CreateReportRequest) {
			r.Metrics = metrics("outreach", report.MetricWeeks{Week1: n("-1")})
		}, "week1"},
		{"three decimals", func(r *report.CreateReportRequest) {
			r.Metrics = metrics("outreach", report.MetricWeeks{Week4: n("1.234")})
		}, "week4"},
		{"above NUMERIC(12,2)", func(r *report.CreateReportRequest) {
			r.Metrics = metrics("outreach", report.MetricWeeks{Week3: n("10000000000")})
		}, "week3"},
		{"exponent form", func(r *report.CreateReportRequest) {
			r.Metrics = metrics("outreach", report.MetricWeeks{Week1: n("1e3")})
		}, "week1"},
		{"not a number", func(r *report.CreateReportRequest) {
			r.Metrics = metrics("outreach", report.MetricWeeks{Week1: n("abc")})
		}, "week1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreate()
			tt.mutate(&req)

			err := v.Validate(req)

			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			ae, ok := apperror.As(err)
			require.True(t, ok, "want *apperror.AppError, got %T", err)
			assert.Equal(t, 422, ae.HTTPStatus)
			assert.Contains(t, ae.Details, tt.wantField)
		})
	}
}

func TestUpdateReportRequest_SharesCreateRules(t *testing.T) {
	err := validator.New().Validate(report.UpdateReportRequest{Month: "Smarch"})
	require.Error(t, err)
}

// D-6: one Go key list is the source of truth. The DTO oneof tag must match it.
func TestCreateReportRequest_MetricTagMatchesMetricKeys(t *testing.T) {
	f, ok := reflect.TypeOf(report.CreateReportRequest{}).FieldByName("Metrics")
	require.True(t, ok)
	m := regexp.MustCompile(`oneof=([a-z_ ]+),endkeys`).FindStringSubmatch(f.Tag.Get("validate"))
	require.Len(t, m, 2, "Metrics tag must contain a oneof list")

	assert.Equal(t, report.MetricKeys, strings.Fields(m[1]))
}

func TestCreateReportRequest_MonthTagMatchesMonths(t *testing.T) {
	f, ok := reflect.TypeOf(report.CreateReportRequest{}).FieldByName("Month")
	require.True(t, ok)
	m := regexp.MustCompile(`oneof=([A-Za-z ]+)`).FindStringSubmatch(f.Tag.Get("validate"))
	require.Len(t, m, 2)

	assert.Equal(t, report.Months, strings.Fields(m[1]))
}

func TestReport_ToResponse_OwnerState(t *testing.T) {
	deleted := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	r := report.Report{OwnerStatus: "deceased", OwnerDeletedAt: &deleted}
	resp := r.ToResponse()
	assert.Equal(t, "deceased", resp.OwnerStatus)
	assert.Equal(t, &deleted, resp.OwnerDeletedAt)
	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"owner_status":"deceased"`)
	assert.Contains(t, string(raw), `"owner_deleted_at":"2026-10-21T09:00:00Z"`)
	r.OwnerDeletedAt = nil
	raw, err = json.Marshal(r.ToResponse())
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"owner_deleted_at"`)
}
