package repository

import (
	"context"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// UserRepository defines the interface for user data access.
type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	CreateWithPassword(ctx context.Context, email, name, hash string) (*model.User, error)
	Create(ctx context.Context, q Querier, n model.NewUser) (*model.User, error)
	UpdateProfile(ctx context.Context, q Querier, id int64, p model.UserProfile) (*model.User, error)
	List(ctx context.Context, emailFilter string, limit int) ([]model.User, error)
	ListWithPagination(ctx context.Context, emailFilter string, page, limit int) ([]model.User, int64, error)
	MarkEmailVerified(ctx context.Context, userID int64) error
	UpdatePassword(ctx context.Context, userID int64, hash string) error
	UpdatePasswordKeepSession(ctx context.Context, q Querier, userID int64, hash, keepToken string) (int64, error)
	RecordFailedLogin(ctx context.Context, userID int64, maxAttempts int, lockDuration time.Duration) error
	ResetLoginAttempts(ctx context.Context, userID int64) error
	UpdateRole(ctx context.Context, userID, roleID int64) error
	GetWithRole(ctx context.Context, id int64) (*model.User, error)
}

// RoleRepository defines the interface for role data access.
type RoleRepository interface {
	Create(ctx context.Context, req model.CreateRoleRequest) (*model.Role, error)
	GetByID(ctx context.Context, id int64) (*model.Role, error)
	GetByName(ctx context.Context, name string) (*model.Role, error)
	List(ctx context.Context) ([]model.Role, error)
	ListWithPagination(ctx context.Context, page, limit int) ([]model.Role, int64, error)
	Update(ctx context.Context, id int64, req model.UpdateRoleRequest) (*model.Role, error)
	Delete(ctx context.Context, id int64) error
	GetPermissions(ctx context.Context, roleID int64) ([]model.Permission, error)
	AssignPermission(ctx context.Context, roleID, permissionID int64) error
	RevokePermission(ctx context.Context, roleID, permissionID int64) error
}

// PermissionRepository defines the interface for permission data access.
type PermissionRepository interface {
	Create(ctx context.Context, name, resource, action string) (*model.Permission, error)
	GetByID(ctx context.Context, id int64) (*model.Permission, error)
	GetByName(ctx context.Context, name string) (*model.Permission, error)
	List(ctx context.Context) ([]model.Permission, error)
	ListWithPagination(ctx context.Context, resource string, page, limit int) ([]model.Permission, int64, error)
	ListByResource(ctx context.Context, resource string) ([]model.Permission, error)
	Delete(ctx context.Context, id int64) error
}

// VerificationRepository defines the interface for verification token access.
type VerificationRepository interface {
	Create(ctx context.Context, userID int64) (*model.VerificationToken, error)
	GetByToken(ctx context.Context, token string) (*model.VerificationToken, error)
	MarkUsed(ctx context.Context, id int64) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// PasswordResetRepository defines the interface for password reset token access.
type PasswordResetRepository interface {
	Create(ctx context.Context, userID int64) (*model.PasswordReset, error)
	GetByToken(ctx context.Context, token string) (*model.PasswordReset, error)
	MarkUsed(ctx context.Context, id int64) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// ReportRepository defines the interface for report data access. Get returns
// (nil, nil) for a missing or soft-deleted report; Update, SoftDelete and
// ToggleComplete return an error wrapping ErrNotFound instead. year is
// resolved by the service (the request may omit it).
type ReportRepository interface {
	Create(ctx context.Context, q Querier, userID int64, year int, req model.CreateReportRequest) (*model.Report, error)
	Get(ctx context.Context, q Querier, id int64) (*model.Report, error)
	List(ctx context.Context, p model.ListReportsParams, pg pagination.Params) ([]*model.Report, int64, error)
	Update(ctx context.Context, q Querier, id int64, year int, req model.UpdateReportRequest) (*model.Report, error)
	SoftDelete(ctx context.Context, q Querier, id int64) error
	ToggleComplete(ctx context.Context, q Querier, id, approverID int64) (*model.Report, error)
}

// StatsRepository defines read-only counts for the admin dashboard.
type StatsRepository interface {
	Counts(ctx context.Context, month string, year int) (model.AdminStats, error)
}

// AuditRepository defines the interface for writing audit rows. Pass the
// transaction of the change being audited.
type AuditRepository interface {
	Insert(ctx context.Context, q Querier, e model.AuditLog) error
}

// Compile-time checks: implementations must satisfy interfaces.
var (
	_ UserRepository          = (*UserRepo)(nil)
	_ RoleRepository          = (*RoleRepo)(nil)
	_ PermissionRepository    = (*PermissionRepo)(nil)
	_ VerificationRepository  = (*VerificationRepo)(nil)
	_ PasswordResetRepository = (*PasswordResetRepo)(nil)
	_ ReportRepository        = (*ReportRepo)(nil)
	_ AuditRepository         = (*AuditRepo)(nil)
	_ StatsRepository         = (*StatsRepo)(nil)
	_ AuditRepository         = (*MockAuditRepo)(nil)
	_ ReportRepository        = (*MockReportRepo)(nil)
)
