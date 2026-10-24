package auth

// Swagger names are pinned (@name) to the ones these types had in package model,
// so the generated frontend types do not change with the package.

// MeResponse is the signed-in user's identity, role and permissions, so a client
// can show or hide actions. The server still enforces every permission.
type MeResponse struct {
	UserID      int64    `json:"user_id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	// Report form defaults; empty when the admin has not set them.
	AreaAssignment string `json:"area_assignment"`
	ChurchName     string `json:"church_name"`
	// AvatarURL is empty when the user has no profile picture.
	AvatarURL string `json:"avatar_url"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.MeResponse

// ChangePasswordRequest is the body of POST /auth/change-password. The 72-byte
// limit on the new password is checked in the service, because bcrypt counts bytes
// and the validator counts characters.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,strongpass"`
	// ConfirmPassword must equal NewPassword exactly. It is checked before any service
	// call, so a typo never counts as a failed current-password attempt.
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=NewPassword"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.ChangePasswordRequest

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.LoginRequest
