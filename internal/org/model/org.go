// Package model holds the tables of the Enterprise Org feature (meta-repo
// docs/enterprise-org-prd.md §7.2): organizations, departments, org roles,
// invites, alerts and the audit log.
//
// 🔴 This package must not import the platform "model" package. model/main.go
// imports it to run the migration, so the reverse import is a cycle. Logic
// that needs platform tables (users, tokens) lives in ../service.
package model

import "gorm.io/gorm"

// Role scopes: how far the primitives of a role reach.
const (
	ScopeOrg  = "org"  // the whole organization
	ScopeDept = "dept" // only the departments the member manages
	ScopeSelf = "self" // only the member's own keys and usage (preset staff)
)

// Organization is one customer company. It holds no money of its own: the
// company wallet is the owner's account quota (PRD §7.3).
type Organization struct {
	Id          int    `json:"id"`
	Name        string `json:"name" gorm:"type:varchar(64);not null"`
	OwnerUserId int    `json:"owner_user_id" gorm:"not null;uniqueIndex"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
}

// TableName pins the PRD's table name.
func (Organization) TableName() string { return "organizations" }

// Department is a pure structural unit: it scopes managers and groups
// reports, and carries no permissions of its own (PRD D9).
type Department struct {
	Id        int            `json:"id"`
	OrgId     int            `json:"org_id" gorm:"not null;index"`
	Name      string         `json:"name" gorm:"type:varchar(64);not null"`
	IsDefault bool           `json:"is_default"`                         // the catch-all department; cannot be deleted
	ParentId  int            `json:"parent_id" gorm:"default:0"`         // always 0 in v1, reserved for a hierarchy
	PresetKey *string        `json:"preset_key" gorm:"type:varchar(32)"` // starter-pack entry it came from; null if user-created
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName pins the PRD's table name.
func (Department) TableName() string { return "departments" }

// OrgRole is a bundle of permission primitives. Rows with OrgId 0 are the five
// platform presets shared by every organization; other rows are custom roles.
type OrgRole struct {
	Id          int            `json:"id"`
	OrgId       int            `json:"org_id" gorm:"index"`
	Name        string         `json:"name" gorm:"type:varchar(64);not null"`
	Scope       string         `json:"scope" gorm:"type:varchar(8);not null"`
	Permissions string         `json:"permissions" gorm:"type:text"` // comma-separated primitives
	IsPreset    bool           `json:"is_preset"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName pins the PRD's table name.
func (OrgRole) TableName() string { return "org_roles" }

// DepartmentManager gives one member dept-scoped reach over one department;
// a member may manage several.
type DepartmentManager struct {
	DepartmentId int `json:"department_id" gorm:"primaryKey;autoIncrement:false"`
	UserId       int `json:"user_id" gorm:"primaryKey;autoIncrement:false;index"`
}

// TableName pins the PRD's table name.
func (DepartmentManager) TableName() string { return "department_managers" }

// OrgInvite is a join code that admits a new member with a fixed role and
// department.
type OrgInvite struct {
	Id           int    `json:"id"`
	OrgId        int    `json:"org_id" gorm:"not null;index"`
	Code         string `json:"code" gorm:"type:varchar(64);not null;uniqueIndex"`
	RoleId       int    `json:"role_id" gorm:"not null"`
	DepartmentId int    `json:"department_id" gorm:"not null"`
	ExpiresTime  int64  `json:"expires_time" gorm:"bigint"`
}

// TableName pins the PRD's table name.
func (OrgInvite) TableName() string { return "org_invites" }

// OrgAlert is one anomaly-rule hit on a key. Alerts notify and never block
// traffic (PRD D13).
type OrgAlert struct {
	Id          int    `json:"id"`
	OrgId       int    `json:"org_id" gorm:"not null;index"`
	Rule        string `json:"rule" gorm:"type:varchar(16);not null"` // spike | offhours | new_ip
	TokenId     int    `json:"token_id" gorm:"index"`
	UserId      int    `json:"user_id"`
	Detail      string `json:"detail" gorm:"type:text"` // JSON
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	AckedBy     int    `json:"acked_by" gorm:"default:0"`
}

// TableName pins the PRD's table name.
func (OrgAlert) TableName() string { return "org_alerts" }

// OrgAuditLog is one append-only record of an organization management action
// (PRD D17). It is kept apart from the global logs table, which platform
// admins can bulk-delete.
type OrgAuditLog struct {
	Id          int    `json:"id"`
	OrgId       int    `json:"org_id" gorm:"not null;index"`
	ActorUserId int    `json:"actor_user_id" gorm:"not null"`
	Action      string `json:"action" gorm:"type:varchar(64);not null"` // a primitive, or the name of an inherent power
	TargetType  string `json:"target_type" gorm:"type:varchar(32)"`
	TargetId    int    `json:"target_id"`
	Detail      string `json:"detail" gorm:"type:text"` // JSON: the values before and after
	Ip          string `json:"ip" gorm:"type:varchar(64)"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
}

// TableName pins the PRD's table name.
func (OrgAuditLog) TableName() string { return "org_audit_logs" }
