package service

import (
	"errors"
	"strings"
	"unicode/utf8"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// DepartmentNameMaxLength is the longest department name, in characters.
const DepartmentNameMaxLength = 64

var (
	// ErrInvalidDepartmentName means the department name is blank or too long.
	ErrInvalidDepartmentName = errors.New("department name must be 1 to 64 characters")
	// ErrDepartmentExists means the organization already has a department with that name.
	ErrDepartmentExists = errors.New("a department with this name already exists")
	// ErrDepartmentNotFound means the department is not one of this organization's.
	ErrDepartmentNotFound = errors.New("department not found in this organization")
	// ErrDefaultDepartment means the action is not allowed on the default department.
	ErrDefaultDepartment = errors.New("the default department cannot be deleted")
)

// DepartmentView is a department with its head count, as the management page lists it.
type DepartmentView struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	IsDefault   bool   `json:"is_default"`
	MemberCount int    `json:"member_count"`
}

// seedDepartmentsTx creates the preset departments of a new organization and
// returns the id of the default one.
func seedDepartmentsTx(tx *gorm.DB, orgID int, lang string) (int, error) {
	defaultID := 0
	for _, preset := range orgmodel.PresetDepartments {
		key := preset.Key
		department := orgmodel.Department{
			OrgId:     orgID,
			Name:      preset.Name(lang),
			IsDefault: preset.IsDefault,
			PresetKey: &key,
		}
		if err := tx.Create(&department).Error; err != nil {
			return 0, err
		}
		if preset.IsDefault {
			defaultID = department.Id
		}
	}
	return defaultID, nil
}

// normalizeDepartmentName trims a department name and checks its length.
func normalizeDepartmentName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > DepartmentNameMaxLength {
		return "", ErrInvalidDepartmentName
	}
	return name, nil
}

// findDepartment loads one live department of an organization.
func findDepartment(db *gorm.DB, orgID int, departmentID int) (*orgmodel.Department, error) {
	// Find, not First: a miss is an ordinary answer here, not worth a log line.
	var found []orgmodel.Department
	if err := db.Where("id = ? AND org_id = ?", departmentID, orgID).Limit(1).Find(&found).Error; err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, ErrDepartmentNotFound
	}
	return &found[0], nil
}

// defaultDepartmentID returns the id of an organization's default department.
func defaultDepartmentID(db *gorm.DB, orgID int) (int, error) {
	var found []orgmodel.Department
	if err := db.Select("id").Where("org_id = ? AND is_default = ?", orgID, true).
		Order("id").Limit(1).Find(&found).Error; err != nil {
		return 0, err
	}
	if len(found) == 0 {
		return 0, ErrDepartmentNotFound
	}
	return found[0].Id, nil
}

// resolveDepartment turns a requested department into one that exists: zero
// means "not specified" and becomes the default department, anything else must
// be a live department of the organization.
func resolveDepartment(db *gorm.DB, orgID int, departmentID int) (int, error) {
	if departmentID == 0 {
		return defaultDepartmentID(db, orgID)
	}
	department, err := findDepartment(db, orgID, departmentID)
	if err != nil {
		return 0, err
	}
	return department.Id, nil
}

// departmentNameTaken reports whether another live department of the
// organization already carries the name.
func departmentNameTaken(db *gorm.DB, orgID int, name string, exceptID int) (bool, error) {
	var n int64
	err := db.Model(&orgmodel.Department{}).
		Where("org_id = ? AND name = ? AND id <> ?", orgID, name, exceptID).Count(&n).Error
	return n > 0, err
}

// ListDepartments returns the organization's departments, the default one
// first, each with the number of members (service accounts included) in it.
func ListDepartments(db *gorm.DB, actor *Actor) ([]DepartmentView, error) {
	if err := actor.requireManager(); err != nil {
		return nil, err
	}
	var departments []orgmodel.Department
	if err := db.Where("org_id = ?", actor.OrgId).
		Order("is_default DESC").Order("id").Find(&departments).Error; err != nil {
		return nil, err
	}
	var counts []struct {
		DepartmentId int
		Members      int
	}
	if err := db.Model(&platformmodel.User{}).
		Select("department_id, COUNT(*) AS members").
		Where("org_id = ?", actor.OrgId).
		Group("department_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	membersIn := make(map[int]int, len(counts))
	for _, count := range counts {
		membersIn[count.DepartmentId] = count.Members
	}
	views := make([]DepartmentView, 0, len(departments))
	for _, department := range departments {
		views = append(views, DepartmentView{
			Id:          department.Id,
			Name:        department.Name,
			IsDefault:   department.IsDefault,
			MemberCount: membersIn[department.Id],
		})
	}
	return views, nil
}

// CreateDepartment adds a department to the organization.
func CreateDepartment(db *gorm.DB, actor *Actor, rawName string) (*DepartmentView, error) {
	if err := actor.requireManager(); err != nil {
		return nil, err
	}
	name, err := normalizeDepartmentName(rawName)
	if err != nil {
		return nil, err
	}
	taken, err := departmentNameTaken(db, actor.OrgId, name, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrDepartmentExists
	}
	department := orgmodel.Department{OrgId: actor.OrgId, Name: name}
	if err := db.Create(&department).Error; err != nil {
		return nil, err
	}
	return &DepartmentView{Id: department.Id, Name: department.Name}, nil
}

// RenameDepartment renames a department. The default department can be
// renamed like any other; it only cannot be deleted.
func RenameDepartment(db *gorm.DB, actor *Actor, departmentID int, rawName string) error {
	if err := actor.requireManager(); err != nil {
		return err
	}
	name, err := normalizeDepartmentName(rawName)
	if err != nil {
		return err
	}
	department, err := findDepartment(db, actor.OrgId, departmentID)
	if err != nil {
		return err
	}
	taken, err := departmentNameTaken(db, actor.OrgId, name, department.Id)
	if err != nil {
		return err
	}
	if taken {
		return ErrDepartmentExists
	}
	return db.Model(&orgmodel.Department{}).Where("id = ?", department.Id).Update("name", name).Error
}

// DeleteDepartment removes a department. Whoever pointed at it — its members,
// and invite links not yet used — falls back to the default department, which
// is why that one cannot be deleted.
func DeleteDepartment(db *gorm.DB, actor *Actor, departmentID int) error {
	if err := actor.requireManager(); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		department, err := findDepartment(tx, actor.OrgId, departmentID)
		if err != nil {
			return err
		}
		if department.IsDefault {
			return ErrDefaultDepartment
		}
		fallbackID, err := defaultDepartmentID(tx, actor.OrgId)
		if err != nil {
			return err
		}
		if err := tx.Model(&platformmodel.User{}).
			Where("org_id = ? AND department_id = ?", actor.OrgId, department.Id).
			Update("department_id", fallbackID).Error; err != nil {
			return err
		}
		if err := tx.Model(&orgmodel.OrgInvite{}).
			Where("org_id = ? AND department_id = ?", actor.OrgId, department.Id).
			Update("department_id", fallbackID).Error; err != nil {
			return err
		}
		// Nobody can keep managing a department that is gone.
		if err := tx.Where("department_id = ?", department.Id).
			Delete(&orgmodel.DepartmentManager{}).Error; err != nil {
			return err
		}
		return tx.Delete(&orgmodel.Department{}, department.Id).Error
	})
}
