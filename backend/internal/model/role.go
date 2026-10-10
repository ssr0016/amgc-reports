package model

import "time"

// Role represents a job title or access level (e.g., admin, editor, viewer).
type Role struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`

	// Permissions attached to this role (loaded on demand)
	Permissions []Permission `json:"permissions,omitempty"`
}

// Permission represents a granular access right (e.g., users:read, users:write).
type Permission struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Resource  string    `json:"resource" db:"resource"`
	Action    string    `json:"action" db:"action"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// RolePermission is the junction between roles and permissions.
type RolePermission struct {
	RoleID       int64     `json:"role_id" db:"role_id"`
	PermissionID int64     `json:"permission_id" db:"permission_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// ============================================================
// Request DTOs
// ============================================================

// CreateRoleRequest is the payload for creating a new role.
type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=50"`
	Description string `json:"description" validate:"max=255"`
}

// UpdateRoleRequest is the payload for updating a role.
type UpdateRoleRequest struct {
	Name        string `json:"name" validate:"omitempty,min=2,max=50"`
	Description string `json:"description" validate:"omitempty,max=255"`
}

// AssignPermissionRequest is the payload for assigning a permission to a role.
type AssignPermissionRequest struct {
	PermissionID int64 `json:"permission_id" validate:"required"`
}

// ChangeRoleRequest is the payload for changing a user's role.
type ChangeRoleRequest struct {
	RoleID int64 `json:"role_id" validate:"required"`
}

// RoleResponse is the API representation of a role.
type RoleResponse struct {
	ID          int64                `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Permissions []PermissionResponse `json:"permissions,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// PermissionResponse is the API representation of a permission.
type PermissionResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// ToResponse converts Role to RoleResponse.
func (r *Role) ToResponse() RoleResponse {
	perms := make([]PermissionResponse, 0, len(r.Permissions))
	for _, p := range r.Permissions {
		perms = append(perms, p.ToResponse())
	}
	return RoleResponse{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Permissions: perms,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// ToResponse converts Permission to PermissionResponse.
func (p *Permission) ToResponse() PermissionResponse {
	return PermissionResponse{
		ID:       p.ID,
		Name:     p.Name,
		Resource: p.Resource,
		Action:   p.Action,
	}
}
