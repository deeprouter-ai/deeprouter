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
	// AlertSettings is what the organization changed about its warnings and
	// alerts, as JSON (see AlertSettings in alerts.go). Empty means every
	// default.
	AlertSettings string `json:"-" gorm:"type:text"`
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

// DepartmentManager adds one department to the ones a member manages. A member
// whose role has department scope always manages the department they belong
// to; these rows are the further ones (PRD D28).
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

// OrgAlert is one thing an organization is told about a key: a warning that
// the key is running out of what it was given, or a hit of an anomaly rule.
// Alerts notify and never block traffic (PRD D13).
type OrgAlert struct {
	Id      int    `json:"id"`
	OrgId   int    `json:"org_id" gorm:"not null;index"`
	Rule    string `json:"rule" gorm:"type:varchar(16);not null"` // one of the AlertRule… names
	TokenId int    `json:"token_id" gorm:"index"`
	// UserId and DepartmentId are the key's holder and the department they
	// were in when the alert was raised. Like the stamp on a usage log line,
	// they are not rewritten when the key or its holder moves, and the
	// department is what the alert list is cut to for a department-scoped role.
	UserId       int `json:"user_id"`
	DepartmentId int `json:"department_id" gorm:"default:0;index"`
	// Cycle and Level are set on a warning only. A key is warned about once
	// per level in each cycle: the calendar month for its monthly limit, and
	// for its quota, what it was given in all — which changes when someone
	// gives it more. The columns carry a prefix because "cycle" and "level"
	// are words of SQL itself on some engines.
	Cycle       string `json:"cycle" gorm:"column:warn_cycle;type:varchar(32);default:''"`
	Level       int    `json:"level" gorm:"column:warn_level;default:0"`
	Detail      string `json:"detail" gorm:"type:text"` // JSON: the numbers behind the alert
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	// NotifiedTime is when the alert went out as a notification; 0 until then.
	NotifiedTime int64 `json:"notified_time" gorm:"bigint;default:0;index"`
	// State is what an owner or admin made of the alert: one of the
	// AlertState… names, AckedBy and AckedTime say who and when.
	State     string `json:"state" gorm:"type:varchar(16);default:''"`
	AckedBy   int    `json:"acked_by" gorm:"default:0"`
	AckedTime int64  `json:"acked_time" gorm:"bigint;default:0"`
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
	Action      string `json:"action" gorm:"type:varchar(64);not null"` // what was done, see audit.go
	TargetType  string `json:"target_type" gorm:"type:varchar(32)"`
	TargetId    int    `json:"target_id"`
	Detail      string `json:"detail" gorm:"type:text"` // JSON: the values before and after
	Ip          string `json:"ip" gorm:"type:varchar(64)"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
}

// TableName pins the PRD's table name.
func (OrgAuditLog) TableName() string { return "org_audit_logs" }
