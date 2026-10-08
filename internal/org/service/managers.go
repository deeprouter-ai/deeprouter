package service

import (
	"slices"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"gorm.io/gorm"
)

// Which departments a member manages (PRD D28). A member whose role has
// department scope always manages the department they belong to, so moving
// them moves what they manage with them. The department_managers table holds
// the further departments an owner or admin added for them — and holds rows
// only for members with such a role: giving them any other role clears them.

// withOwnDepartment returns the departments a member with a department-scoped
// role manages: the one they belong to first, then the added ones.
func withOwnDepartment(own int, added []int) []int {
	managed := []int{own}
	for _, departmentID := range added {
		if !slices.Contains(managed, departmentID) {
			managed = append(managed, departmentID)
		}
	}
	return managed
}

// extraDepartments returns, per member, the departments added to the one they
// belong to, in id order.
func extraDepartments(db *gorm.DB, userIDs []int) (map[int][]int, error) {
	added := map[int][]int{}
	if len(userIDs) == 0 {
		return added, nil
	}
	var rows []orgmodel.DepartmentManager
	if err := db.Where("user_id IN ?", userIDs).Order("department_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		added[row.UserId] = append(added[row.UserId], row.DepartmentId)
	}
	return added, nil
}

// replaceExtraDepartments makes the departments added for a member exactly the
// given ones.
func replaceExtraDepartments(tx *gorm.DB, userID int, departmentIDs []int) error {
	if err := clearExtraDepartments(tx, []int{userID}); err != nil {
		return err
	}
	for _, departmentID := range departmentIDs {
		row := orgmodel.DepartmentManager{DepartmentId: departmentID, UserId: userID}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// clearExtraDepartments removes the departments added for the given members,
// for when their role stops having department scope.
func clearExtraDepartments(tx *gorm.DB, userIDs []int) error {
	if len(userIDs) == 0 {
		return nil
	}
	return tx.Where("user_id IN ?", userIDs).Delete(&orgmodel.DepartmentManager{}).Error
}
