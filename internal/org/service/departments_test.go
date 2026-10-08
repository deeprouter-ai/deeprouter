package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P3 (meta-repo docs/enterprise-org-prd.md), acceptance item
// "组织创建时自动生成预设部门（内容以调研定稿为准），其中默认部门不可删除；owner/admin
// 能新建、改名、删除其余部门，并调整成员的部门归属".

// liveDepartments returns the names of an organization's departments in the
// order the page lists them.
func liveDepartments(t *testing.T, db *gorm.DB, actor *Actor) []string {
	t.Helper()
	departments, err := ListDepartments(db, actor)
	require.NoError(t, err)
	names := make([]string, 0, len(departments))
	for _, department := range departments {
		names = append(names, department.Name)
	}
	return names
}

func TestCreateForOwnerTx_SeedsThePresetDepartments(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		for _, tc := range []struct {
			lang  string
			names []string
		}{
			{"zh-CN", []string{"综合", "技术", "产品", "市场", "销售", "客服"}},
			{"zh-TW", []string{"綜合", "技術", "產品", "市場", "銷售", "客服"}},
			{"en", []string{"General", "Engineering", "Product", "Marketing", "Sales", "Customer Support"}},
			{"", []string{"General", "Engineering", "Product", "Marketing", "Sales", "Customer Support"}},
		} {
			founder := seedUser(t, db, "founder-"+tc.lang, common.RoleCommonUser)
			var org *orgmodel.Organization
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				var err error
				org, err = CreateForOwnerTx(tx, founder.Id, "Acme "+tc.lang, tc.lang)
				return err
			}))

			var departments []orgmodel.Department
			require.NoError(t, db.Where("org_id = ?", org.Id).Order("id").Find(&departments).Error)
			require.Len(t, departments, len(orgmodel.PresetDepartments))
			for i, department := range departments {
				preset := orgmodel.PresetDepartments[i]
				require.Equal(t, tc.names[i], department.Name, "lang %q", tc.lang)
				require.Equal(t, preset.IsDefault, department.IsDefault)
				require.NotNil(t, department.PresetKey)
				require.Equal(t, preset.Key, *department.PresetKey)
				require.Zero(t, department.ParentId, "v1 has one level of departments")
			}

			// The owner sits in the default department, like anyone whose
			// department was never chosen.
			require.Equal(t, defaultDepartment(t, db, org.Id).Id, reloadUser(t, db, founder.Id).DepartmentId)
			require.Equal(t, tc.names[0], defaultDepartment(t, db, org.Id).Name)
		}
	})
}

// A sign-up that fails after the organization row was written must not leave
// departments behind either.
func TestCreateForOwnerTx_DepartmentsRollBackWithTheSignUp(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		founder := seedUser(t, db, "founder", common.RoleCommonUser)
		sentinel := gorm.ErrInvalidTransaction
		err := db.Transaction(func(tx *gorm.DB) error {
			if _, err := CreateForOwnerTx(tx, founder.Id, "Acme", "en"); err != nil {
				return err
			}
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)
		var departments int64
		require.NoError(t, db.Model(&orgmodel.Department{}).Count(&departments).Error)
		require.Zero(t, departments)
	})
}

func TestListDepartments_PutsTheDefaultFirstAndCountsMembers(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		for _, username := range []string{"sam", "taeron"} {
			member := seedMember(t, db, acme, username, orgmodel.RoleStaff)
			require.NoError(t, UpdateMember(db, owner, member.Id, MemberPatch{DepartmentId: intPtr(sales.Id)}))
		}
		bot, err := CreateServiceAccount(db, owner, "CI", sales.Id)
		require.NoError(t, err)
		require.NotZero(t, bot.Id)
		// Somebody else's company has a Sales department too; it must not be counted.
		other := seedOrg(t, db, "Other")
		seedMember(t, db, other, "stranger", orgmodel.RoleStaff)

		departments, err := ListDepartments(db, owner)
		require.NoError(t, err)
		require.Len(t, departments, 6)
		require.Equal(t, "General", departments[0].Name)
		require.True(t, departments[0].IsDefault)
		require.Equal(t, 1, departments[0].MemberCount, "the owner")
		for _, department := range departments[1:] {
			require.False(t, department.IsDefault)
			if department.Id == sales.Id {
				require.Equal(t, 3, department.MemberCount, "two people and a service account")
			} else {
				require.Zero(t, department.MemberCount, department.Name)
			}
		}
	})
}

func TestCreateDepartment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)

		created, err := CreateDepartment(db, owner, "  Research  ")
		require.NoError(t, err)
		require.Equal(t, "Research", created.Name)
		require.False(t, created.IsDefault)
		require.Zero(t, created.MemberCount)
		stored := departmentNamed(t, db, acme.id, "Research")
		require.Equal(t, created.Id, stored.Id)
		require.Nil(t, stored.PresetKey, "a department the company made is not a preset")
		require.Contains(t, liveDepartments(t, db, owner), "Research")

		longest := strings.Repeat("部", DepartmentNameMaxLength)
		_, err = CreateDepartment(db, owner, longest)
		require.NoError(t, err, "the limit counts characters, not bytes")

		for _, bad := range []string{"", "   ", longest + "部"} {
			_, err := CreateDepartment(db, owner, bad)
			require.ErrorIs(t, err, ErrInvalidDepartmentName, "%q", bad)
		}
		for _, taken := range []string{"Research", " Research ", "Sales", "General"} {
			_, err := CreateDepartment(db, owner, taken)
			require.ErrorIs(t, err, ErrDepartmentExists, "%q", taken)
		}

		// Names are only unique inside one company.
		other := seedOrg(t, db, "Other")
		_, err = CreateDepartment(db, actorFor(t, db, other.owner.Id), "Research")
		require.NoError(t, err)
	})
}

func TestRenameDepartment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		general := defaultDepartment(t, db, acme.id)

		require.NoError(t, RenameDepartment(db, owner, sales.Id, "  Revenue "))
		require.Equal(t, "Revenue", departmentNamed(t, db, acme.id, "Revenue").Name)
		require.NotContains(t, liveDepartments(t, db, owner), "Sales")

		// The default department is an ordinary one in every way but deletion.
		require.NoError(t, RenameDepartment(db, owner, general.Id, "Everyone"))
		renamed := defaultDepartment(t, db, acme.id)
		require.Equal(t, "Everyone", renamed.Name)
		require.Equal(t, general.Id, renamed.Id)

		// Saving a department under the name it already has is not a clash.
		require.NoError(t, RenameDepartment(db, owner, sales.Id, "Revenue"))

		require.ErrorIs(t, RenameDepartment(db, owner, sales.Id, "Product"), ErrDepartmentExists)
		require.ErrorIs(t, RenameDepartment(db, owner, sales.Id, "  "), ErrInvalidDepartmentName)
		require.ErrorIs(t, RenameDepartment(db, owner, 424242, "Ghost"), ErrDepartmentNotFound)
	})
}

func TestDeleteDepartment_MovesWhoeverPointedAtItToTheDefault(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		general := defaultDepartment(t, db, acme.id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		product := departmentNamed(t, db, acme.id, "Product")

		seller := seedMember(t, db, acme, "seller", orgmodel.RoleStaff)
		require.NoError(t, UpdateMember(db, owner, seller.Id, MemberPatch{DepartmentId: intPtr(sales.Id)}))
		bot, err := CreateServiceAccount(db, owner, "Sales bot", sales.Id)
		require.NoError(t, err)
		planner := seedMember(t, db, acme, "planner", orgmodel.RoleStaff)
		require.NoError(t, UpdateMember(db, owner, planner.Id, MemberPatch{DepartmentId: intPtr(product.Id)}))
		salesInvite, err := CreateInvite(db, owner, presetRoleID(t, db, orgmodel.RoleStaff), sales.Id)
		require.NoError(t, err)
		productInvite, err := CreateInvite(db, owner, presetRoleID(t, db, orgmodel.RoleStaff), product.Id)
		require.NoError(t, err)
		require.NoError(t, db.Create(&orgmodel.DepartmentManager{DepartmentId: sales.Id, UserId: seller.Id}).Error)
		require.NoError(t, db.Create(&orgmodel.DepartmentManager{DepartmentId: product.Id, UserId: planner.Id}).Error)

		require.NoError(t, DeleteDepartment(db, owner, sales.Id))

		require.NotContains(t, liveDepartments(t, db, owner), "Sales")
		require.Equal(t, general.Id, reloadUser(t, db, seller.Id).DepartmentId)
		require.Equal(t, general.Id, reloadUser(t, db, bot.Id).DepartmentId)
		require.Equal(t, product.Id, reloadUser(t, db, planner.Id).DepartmentId, "other departments are untouched")

		// Two variables: GORM would add the id already loaded into a reused
		// struct to the next query's conditions.
		var moved, untouched orgmodel.OrgInvite
		require.NoError(t, db.First(&moved, salesInvite.Id).Error)
		require.Equal(t, general.Id, moved.DepartmentId, "an unused invite link follows its department")
		require.NoError(t, db.First(&untouched, productInvite.Id).Error)
		require.Equal(t, product.Id, untouched.DepartmentId)

		var managers []orgmodel.DepartmentManager
		require.NoError(t, db.Order("department_id").Find(&managers).Error)
		require.Equal(t, []orgmodel.DepartmentManager{{DepartmentId: product.Id, UserId: planner.Id}}, managers)

		// The row is kept for old usage records to point at; it is only hidden.
		var kept orgmodel.Department
		require.NoError(t, db.Unscoped().First(&kept, sales.Id).Error)
		require.True(t, kept.DeletedAt.Valid)

		// Its name is free again, and it cannot be deleted twice.
		_, err = CreateDepartment(db, owner, "Sales")
		require.NoError(t, err)
		require.ErrorIs(t, DeleteDepartment(db, owner, sales.Id), ErrDepartmentNotFound)
	})
}

func TestDeleteDepartment_RefusesTheDefaultDepartment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		general := defaultDepartment(t, db, acme.id)

		require.ErrorIs(t, DeleteDepartment(db, owner, general.Id), ErrDefaultDepartment)
		// Renaming it does not make it deletable.
		require.NoError(t, RenameDepartment(db, owner, general.Id, "Everyone"))
		require.ErrorIs(t, DeleteDepartment(db, owner, general.Id), ErrDefaultDepartment)

		require.Equal(t, general.Id, defaultDepartment(t, db, acme.id).Id)
		require.Equal(t, general.Id, reloadUser(t, db, acme.owner.Id).DepartmentId)
		var live int64
		require.NoError(t, db.Model(&orgmodel.Department{}).Where("org_id = ?", acme.id).Count(&live).Error)
		require.EqualValues(t, len(orgmodel.PresetDepartments), live)
		require.ErrorIs(t, DeleteDepartment(db, owner, 424242), ErrDepartmentNotFound)
	})
}

// Deleting a department moves rows of users, which is an upstream table. The
// global role and everything else about those users must come out untouched.
func TestDeleteDepartment_OnlyMovesTheDepartmentColumn(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, acme.owner.Id)
		sales := departmentNamed(t, db, acme.id, "Sales")
		seller := seedMember(t, db, acme, "seller", orgmodel.RoleStaff)
		require.NoError(t, UpdateMember(db, owner, seller.Id, MemberPatch{DepartmentId: intPtr(sales.Id)}))
		before := reloadUser(t, db, seller.Id)

		require.NoError(t, DeleteDepartment(db, owner, sales.Id))

		want := before
		want.DepartmentId = defaultDepartment(t, db, acme.id).Id
		require.Equal(t, want, reloadUser(t, db, seller.Id))
		var commonUsers int64
		require.NoError(t, db.Model(&platformmodel.User{}).Where("role = ?", common.RoleCommonUser).Count(&commonUsers).Error)
		var everyone int64
		require.NoError(t, db.Model(&platformmodel.User{}).Count(&everyone).Error)
		require.Equal(t, everyone, commonUsers)
	})
}
