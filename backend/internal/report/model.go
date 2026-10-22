package report

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Swagger names are pinned (@name) to the ones these types had in package model,
// so the generated frontend types do not change with the package.

// Report maps to the reports table.
type Report struct {
	ID              int64      `json:"id" db:"id"`
	CreatedBy       int64      `json:"created_by" db:"created_by"`
	Month           string     `json:"month" db:"month"`
	Year            int16      `json:"year" db:"year"`
	Worker          *string    `json:"worker,omitempty" db:"worker"`
	AreaAssignment  *string    `json:"area_assignment,omitempty" db:"area_assignment"`
	ChurchName      *string    `json:"church_name,omitempty" db:"church_name"`
	Names           *string    `json:"names,omitempty" db:"names"`
	NarrativeReport *string    `json:"narrative_report,omitempty" db:"narrative_report"`
	Challenges      *string    `json:"challenges,omitempty" db:"challenges"`
	PrayerRequest   *string    `json:"prayer_request,omitempty" db:"prayer_request"`
	Completed       bool       `json:"completed" db:"completed"`
	CompletedAt     *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	CompletedBy     *int64     `json:"completed_by,omitempty" db:"completed_by"`
	DeletedAt       *time.Time `json:"-" db:"deleted_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`

	// Owner state is loaded from users, independently of the report's deletion state.
	OwnerStatus    string     `json:"-"`
	OwnerDeletedAt *time.Time `json:"-"`

	// Metrics is loaded on demand (not a column): metric key -> week_no -> value.
	Metrics map[string]map[int]pgtype.Numeric `json:"-"`
}

// ReportMetric maps to the report_metrics table.
type ReportMetric struct {
	ReportID int64          `db:"report_id"`
	Metric   string         `db:"metric"`
	WeekNo   int16          `db:"week_no"`
	Value    pgtype.Numeric `db:"value"`
}

// MetricWeeks holds the five weekly values of one metric (Express: week1..week5).
// A nil week is absent ("not reported", D-5), which is not the same as zero.
// Values are JSON numbers backed by json.Number (D-4): at most 2 decimals, 0 to
// 9,999,999,999.99.
type MetricWeeks struct {
	Week1 *json.Number `json:"week1,omitempty" validate:"omitempty,decimal12_2" swaggertype:"number"`
	Week2 *json.Number `json:"week2,omitempty" validate:"omitempty,decimal12_2" swaggertype:"number"`
	Week3 *json.Number `json:"week3,omitempty" validate:"omitempty,decimal12_2" swaggertype:"number"`
	Week4 *json.Number `json:"week4,omitempty" validate:"omitempty,decimal12_2" swaggertype:"number"`
	Week5 *json.Number `json:"week5,omitempty" validate:"omitempty,decimal12_2" swaggertype:"number"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.MetricWeeks

// NonNil returns the supplied weeks keyed by week_no (1..5).
func (m MetricWeeks) NonNil() map[int]json.Number {
	out := make(map[int]json.Number, 5)
	for i, w := range [5]*json.Number{m.Week1, m.Week2, m.Week3, m.Week4, m.Week5} {
		if w != nil {
			out[i+1] = *w
		}
	}
	return out
}

func (m *MetricWeeks) set(weekNo int, v json.Number) {
	switch weekNo {
	case 1:
		m.Week1 = &v
	case 2:
		m.Week2 = &v
	case 3:
		m.Week3 = &v
	case 4:
		m.Week4 = &v
	case 5:
		m.Week5 = &v
	}
}

// CreateReportRequest is the body of POST /reports; created_by and completed are
// never accepted from the client. Empty strings are stored as NULL (D-8).
type CreateReportRequest struct {
	Month           string                 `json:"month" validate:"required,oneof=January February March April May June July August September October November December"`
	Year            *int                   `json:"year" validate:"omitempty,gte=2000,lte=2100"`
	Worker          string                 `json:"worker" validate:"max=150"`
	AreaAssignment  string                 `json:"area_assignment" validate:"max=150"`
	ChurchName      string                 `json:"church_name" validate:"max=150"`
	Names           string                 `json:"names" validate:"max=5000"`
	NarrativeReport string                 `json:"narrative_report" validate:"max=10000"`
	Challenges      string                 `json:"challenges" validate:"max=10000"`
	PrayerRequest   string                 `json:"prayer_request" validate:"max=10000"`
	Metrics         map[string]MetricWeeks `json:"metrics" validate:"dive,keys,oneof=worship_service sunday_school prayer_meeting bible_studies mens_fellowship womens_fellowship youth_fellowship children_fellowship tithes_offering home_visited bible_study_group_led sermon_preached person_newly_contacted person_followed_up person_evangelized outreach training leadership baptism other family_day,endkeys"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.CreateReportRequest

// UpdateReportRequest is the body of PUT /reports/:id. It replaces the metric set.
type UpdateReportRequest CreateReportRequest // @name github_com_ssr0016_pastoral-reports_backend_internal_model.UpdateReportRequest

// ReportResponse is the API representation (bare object, per D-9).
type ReportResponse struct {
	OwnerStatus     string                 `json:"owner_status"`
	OwnerDeletedAt  *time.Time             `json:"owner_deleted_at,omitempty"`
	ID              int64                  `json:"id"`
	CreatedBy       int64                  `json:"created_by"`
	Month           string                 `json:"month"`
	Year            int                    `json:"year"`
	Worker          string                 `json:"worker,omitempty"`
	AreaAssignment  string                 `json:"area_assignment,omitempty"`
	ChurchName      string                 `json:"church_name,omitempty"`
	Metrics         map[string]MetricWeeks `json:"metrics"`
	Names           string                 `json:"names,omitempty"`
	NarrativeReport string                 `json:"narrative_report,omitempty"`
	Challenges      string                 `json:"challenges,omitempty"`
	PrayerRequest   string                 `json:"prayer_request,omitempty"`
	Completed       bool                   `json:"completed"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	CompletedBy     *int64                 `json:"completed_by,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.ReportResponse

// ReportMetadata is the result of approve/unapprove. It carries no report text:
// the caller may hold reports:approve without being allowed to read the report (D-1).
type ReportMetadata struct {
	ID          int64      `json:"id"`
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CompletedBy *int64     `json:"completed_by,omitempty"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.ReportMetadata

// ExportReportsRequest is the body of POST /admin/reports/excel: one month.
type ExportReportsRequest struct {
	Month string `json:"month" validate:"required,oneof=January February March April May June July August September October November December"`
	Year  int    `json:"year" validate:"required,gte=2000,lte=2100"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.ExportReportsRequest

// ExcelPreviewRequest is the body of POST /admin/reports/excel/preview: one
// report (report_id) or one month (month and year), never both (D-A15).
type ExcelPreviewRequest struct {
	ReportID int64  `json:"report_id,omitempty" validate:"required_without=Month,excluded_with=Month,omitempty,gt=0"`
	Month    string `json:"month,omitempty" validate:"required_without=ReportID,required_with=Year,omitempty,oneof=January February March April May June July August September October November December"`
	Year     int    `json:"year,omitempty" validate:"required_with=Month,excluded_with=ReportID,omitempty,gte=2000,lte=2100"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.ExcelPreviewRequest

// ExcelPreview is the workbook an export would build, as text: one sheet per
// report, in the export's order.
type ExcelPreview struct {
	Count  int            `json:"count"`
	Sheets []PreviewSheet `json:"sheets"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.ExcelPreview

// PreviewSheet is one sheet's name and its cell text, row by row, as shown. A
// report sheet is always A1:G42 (42 × 7) and a merged range's text is in its
// top-left cell; the bulk Report Dashboard is its title, header and one row
// per account, 6 columns wide (D-A16, D-A19).
type PreviewSheet struct {
	Name string     `json:"name"`
	Rows [][]string `json:"rows"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.PreviewSheet

// The Report Dashboard statuses (D-A16).
const (
	StatusApproved = "Approved"
	StatusPending  = "Pending"
	StatusNotFiled = "Not Filed"
)

// DashboardRow is one church account on the bulk export's Report Dashboard
// sheet (D-A16, D-A19): the account's church, its report's month and year, and
// its report status. Month is "" and Year 0 when the account has not filed.
type DashboardRow struct {
	Church string
	Month  string
	Year   int
	Status string
}

// Export is a generated .xlsx file and its download name.
type Export struct {
	Filename string
	Data     []byte
}

// ListReportsParams are the list filters (page and limit come from pkg/pagination).
type ListReportsParams struct {
	Month  string // optional
	Year   int    // optional, 0 = any
	Worker string
	Area   string
	Church string

	// CreatedBy limits the list to one owner (D-1); 0 = any owner.
	CreatedBy int64

	// Completed limits the list to approved (true) or pending (false) reports; nil = any.
	Completed *bool
}

// ToResponse converts a report to its API form. A NUMERIC value is emitted in
// its text form (D-4), so Postgres' "120.00" stays "120.00".
func (r *Report) ToResponse() ReportResponse {
	metrics := make(map[string]MetricWeeks, len(r.Metrics))
	for key, weeks := range r.Metrics {
		var mw MetricWeeks
		for weekNo, n := range weeks {
			if v, ok := numericText(n); ok {
				mw.set(weekNo, v)
			}
		}
		metrics[key] = mw
	}

	return ReportResponse{
		OwnerStatus:     r.OwnerStatus,
		OwnerDeletedAt:  r.OwnerDeletedAt,
		ID:              r.ID,
		CreatedBy:       r.CreatedBy,
		Month:           r.Month,
		Year:            int(r.Year),
		Worker:          deref(r.Worker),
		AreaAssignment:  deref(r.AreaAssignment),
		ChurchName:      deref(r.ChurchName),
		Metrics:         metrics,
		Names:           deref(r.Names),
		NarrativeReport: deref(r.NarrativeReport),
		Challenges:      deref(r.Challenges),
		PrayerRequest:   deref(r.PrayerRequest),
		Completed:       r.Completed,
		CompletedAt:     r.CompletedAt,
		CompletedBy:     r.CompletedBy,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func numericText(n pgtype.Numeric) (json.Number, bool) {
	v, err := n.Value()
	if err != nil || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return json.Number(s), ok
}
