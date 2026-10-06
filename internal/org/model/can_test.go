package model

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Enterprise Org P4 (meta-repo docs/enterprise-org-prd.md), acceptance item
// "权限判定函数有真值表测试：穷举权限原语 × 五种预设角色 × 作用域，每个原语至少一条
// “无权限被拒”用例". Everything here runs without a database: Can is a pure
// function, and that is the point of it.

// The cast of the truth table. Sales and Support are departments the manager
// manages, Product one they do not; colleague is somebody else's user id.
const (
	self      = 7
	colleague = 8
	sales     = 11
	support   = 12
	product   = 13
)

// presetSubject is a member holding the preset role of that name — built from
// PresetRoles, because the seed data is half of what the table tests.
func presetSubject(t *testing.T, name string) Subject {
	t.Helper()
	for _, preset := range PresetRoles {
		if preset.Name != name {
			continue
		}
		subject := Subject{
			UserId:      self,
			IsOwner:     name == RoleOwner,
			IsAdmin:     name == RoleAdmin,
			Scope:       preset.Scope,
			Permissions: preset.Permissions,
		}
		if preset.Scope == ScopeDept {
			subject.Departments = []int{sales, support}
		}
		return subject
	}
	t.Fatalf("no preset role named %s", name)
	return Subject{}
}

// customSubject is a member holding a custom role.
func customSubject(scope string, departments []int, permissions ...string) Subject {
	return Subject{UserId: self, Scope: scope, Permissions: permissions, Departments: departments}
}

// truthTable is the PRD §2 role table, one row per primitive, written out by
// hand. It must not be derived from PresetRoles: a table built from the seed
// data would agree with any mistake made there.
//
// The columns are the five preset roles crossed with their scope: a role that
// reaches the whole organization has no "outside", a manager is asked about a
// department they manage and about one they do not, and staff about what is
// their own and about a colleague's.
var truthTable = []struct {
	primitive  string
	owner      bool
	admin      bool
	managerIn  bool
	managerOut bool
	staffOwn   bool
	staffOther bool
	readonly   bool
}{
	{"key.read", true, true, true, false, true, false, true},
	{"key.create", true, true, false, false, false, false, false},
	{"key.update", true, true, false, false, false, false, false},
	{"key.assign", true, true, true, false, false, false, false},
	{"key.rotate", true, true, false, false, false, false, false},
	{"key.freeze", true, true, false, false, false, false, false},
	{"key.delete", true, true, false, false, false, false, false},
	{"member.read", true, true, true, false, false, false, true},
	{"member.invite", true, true, true, false, false, false, false},
	{"member.remove", true, true, false, false, false, false, false},
	{"usage.read", true, true, true, false, true, false, true},
	{"alert.read", true, true, true, false, false, false, true},
	{"audit.read", true, true, false, false, false, false, true},
}

func TestCan_TruthTableOfThePresetRoles(t *testing.T) {
	// One row per primitive, in the order of Primitives: a primitive added
	// later fails here until someone decides who gets it.
	covered := make([]string, 0, len(truthTable))
	for _, row := range truthTable {
		covered = append(covered, row.primitive)
	}
	require.Equal(t, Primitives, covered)

	// Targets a role that reaches the whole organization must all be able to
	// act on: any department, anybody's, and the organization as a whole.
	anywhere := []Target{
		{DepartmentId: sales, UserId: colleague},
		{DepartmentId: product, UserId: colleague},
		{DepartmentId: product},
		{},
	}
	inside := []Target{{DepartmentId: sales, UserId: colleague}, {DepartmentId: support}}
	outside := []Target{{DepartmentId: product, UserId: colleague}, {DepartmentId: product}, {}}
	own := []Target{{DepartmentId: sales, UserId: self}}
	// A colleague in the same department is still somebody else.
	someoneElses := []Target{{DepartmentId: sales, UserId: colleague}, {DepartmentId: sales}, {}}

	for _, row := range truthTable {
		for _, column := range []struct {
			name    string
			subject Subject
			targets []Target
			want    bool
		}{
			{"owner", presetSubject(t, RoleOwner), anywhere, row.owner},
			{"admin", presetSubject(t, RoleAdmin), anywhere, row.admin},
			{"manager inside", presetSubject(t, RoleManager), inside, row.managerIn},
			{"manager outside", presetSubject(t, RoleManager), outside, row.managerOut},
			{"staff own", presetSubject(t, RoleStaff), own, row.staffOwn},
			{"staff other", presetSubject(t, RoleStaff), someoneElses, row.staffOther},
			{"readonly", presetSubject(t, RoleReadonly), anywhere, row.readonly},
		} {
			for _, target := range column.targets {
				require.Equal(t, column.want, Can(column.subject, row.primitive, target),
					"%s, %s, target %+v", row.primitive, column.name, target)
			}
		}
	}
}

// The acceptance item's second half: every primitive has someone who is
// refused it — in the table above, not by accident of this test.
func TestCan_EveryPrimitiveHasARefusal(t *testing.T) {
	for _, row := range truthTable {
		refused := 0
		for _, allowed := range []bool{row.owner, row.admin, row.managerIn, row.managerOut, row.staffOwn, row.staffOther, row.readonly} {
			if !allowed {
				refused++
			}
		}
		require.NotZero(t, refused, row.primitive)
		// Staff hold no primitives, so somebody else's is always out of reach.
		require.False(t, Can(presetSubject(t, RoleStaff), row.primitive, Target{DepartmentId: sales, UserId: colleague}), row.primitive)
	}
}

// PRD §2: readonly sees everything and changes nothing.
func TestCan_ReadonlyReadsTheWholeOrganizationAndWritesNothing(t *testing.T) {
	readonly := presetSubject(t, RoleReadonly)
	everywhere := []Target{{DepartmentId: sales, UserId: colleague}, {DepartmentId: product}, {}}
	for _, primitive := range Primitives {
		isRead := strings.HasSuffix(primitive, ".read")
		for _, target := range everywhere {
			require.Equal(t, isRead, Can(readonly, primitive, target), "%s on %+v", primitive, target)
		}
	}
	for _, power := range InherentPowers {
		require.False(t, Can(readonly, power.Name, Target{}), power.Name)
	}
}

// PRD §2: managing the organization itself comes with who you are. The table
// is written out by hand, like the one above.
func TestCan_InherentPowersBelongToTheOwnerAndAdmins(t *testing.T) {
	table := []struct {
		power string
		owner bool
		admin bool
	}{
		{PowerRoles, true, true},
		{PowerDepartments, true, true},
		{PowerServiceAccounts, true, true},
		{PowerSettings, true, true},
		{PowerAlerts, true, true},
		{PowerAdmins, true, false},
		{PowerWallet, true, false},
		{PowerOwnership, true, false},
	}
	listed := make([]string, 0, len(table))
	for _, row := range table {
		listed = append(listed, row.power)
	}
	defined := make([]string, 0, len(InherentPowers))
	for _, power := range InherentPowers {
		defined = append(defined, power.Name)
	}
	require.Equal(t, defined, listed, "an inherent power added later belongs in this table")

	// A custom role can hold every primitive across the whole organization and
	// still not run it.
	everything := customSubject(ScopeOrg, nil, Primitives...)
	for _, row := range table {
		for _, target := range []Target{{}, {DepartmentId: sales, UserId: colleague}} {
			require.Equal(t, row.owner, Can(presetSubject(t, RoleOwner), row.power, target), "%s, owner", row.power)
			require.Equal(t, row.admin, Can(presetSubject(t, RoleAdmin), row.power, target), "%s, admin", row.power)
			for _, role := range []string{RoleManager, RoleStaff, RoleReadonly} {
				require.False(t, Can(presetSubject(t, role), row.power, target), "%s, %s", row.power, role)
			}
			require.False(t, Can(everything, row.power, target), "%s, a custom role with every primitive", row.power)
		}
	}
}

// An inherent power is never a primitive: nobody can list one in a role.
func TestInherentPowers_AreNotPrimitives(t *testing.T) {
	for _, power := range InherentPowers {
		require.NotContains(t, Primitives, power.Name)
		// Even written into a role's permissions by hand, it grants nothing.
		forged := customSubject(ScopeOrg, nil, power.Name)
		require.False(t, Can(forged, power.Name, Target{}), power.Name)
	}
}

// PRD §2: 写权限自动包含同一资源的读权限.
func TestCan_AWriteBringsTheReadOfTheSameResource(t *testing.T) {
	target := Target{DepartmentId: sales, UserId: colleague}
	for _, tc := range []struct {
		held    string
		implied string
	}{
		{"key.assign", "key.read"},
		{"key.delete", "key.read"},
		{"member.invite", "member.read"},
		{"member.remove", "member.read"},
	} {
		subject := customSubject(ScopeOrg, nil, tc.held)
		require.True(t, Can(subject, tc.implied, target), "%s implies %s", tc.held, tc.implied)
		require.True(t, subject.Holds(tc.implied))
	}

	// It goes one way and stays inside the resource.
	reader := customSubject(ScopeOrg, nil, "key.read", "member.read")
	for _, write := range []string{"key.create", "key.assign", "key.delete", "member.invite", "member.remove"} {
		require.False(t, Can(reader, write, target), "a read must not imply %s", write)
	}
	keyAdmin := customSubject(ScopeOrg, nil, "key.assign")
	for _, other := range []string{"member.read", "usage.read", "alert.read", "audit.read", "key.create"} {
		require.False(t, Can(keyAdmin, other, target), "key.assign must not imply %s", other)
	}
}

// A role is stored and reported with the reads its writes bring, in the order
// of the primitive list — the same rule as Can, spelled out as a list.
func TestWithImpliedReads(t *testing.T) {
	for _, tc := range []struct {
		held []string
		want []string
	}{
		{nil, []string{}},
		{[]string{"usage.read"}, []string{"usage.read"}},
		{[]string{"key.assign"}, []string{"key.read", "key.assign"}},
		{[]string{"member.remove", "key.delete"}, []string{"key.read", "key.delete", "member.read", "member.remove"}},
		{[]string{"key.read", "key.freeze", "member.read"}, []string{"key.read", "key.freeze", "member.read"}},
		{Primitives, Primitives},
	} {
		got := WithImpliedReads(tc.held)
		require.Equal(t, tc.want, got, fmt.Sprint(tc.held))
		// What it adds is exactly what Can already granted.
		for _, primitive := range Primitives {
			held := customSubject(ScopeOrg, nil, tc.held...)
			listed := customSubject(ScopeOrg, nil, got...)
			require.Equal(t, Can(held, primitive, Target{}), slices.Contains(got, primitive), "%v, %s", tc.held, primitive)
			require.Equal(t, Can(held, primitive, Target{}), Can(listed, primitive, Target{}), "%v, %s", tc.held, primitive)
		}
	}
	// The presets and the packs already list their reads.
	for _, preset := range PresetRoles {
		require.Equal(t, append([]string{}, preset.Permissions...), WithImpliedReads(preset.Permissions), preset.Name)
	}
	for _, pack := range RolePacks {
		require.Equal(t, pack.Permissions, WithImpliedReads(pack.Permissions), pack.Key)
	}
}

// PRD §2: the scope is the role's, not the primitive's. The same primitive
// reaches the whole organization in one role and a few departments in another.
func TestCan_ADepartmentScopeReachesExactlyTheManagedDepartments(t *testing.T) {
	scoped := customSubject(ScopeDept, []int{sales, support}, "key.freeze", "member.remove", "usage.read")
	for _, primitive := range []string{"key.freeze", "key.read", "member.remove", "member.read", "usage.read"} {
		require.True(t, Can(scoped, primitive, Target{DepartmentId: sales, UserId: colleague}), primitive)
		require.True(t, Can(scoped, primitive, Target{DepartmentId: support}), primitive)
		require.False(t, Can(scoped, primitive, Target{DepartmentId: product, UserId: colleague}), primitive)
		require.False(t, Can(scoped, primitive, Target{}), "%s on the whole organization", primitive)
	}

	// Managing nothing reaches nothing.
	nowhere := customSubject(ScopeDept, nil, "key.freeze")
	require.False(t, Can(nowhere, "key.freeze", Target{DepartmentId: sales}))
	require.False(t, Can(nowhere, "key.freeze", Target{}))

	// The departments are only read for a role with department scope.
	wide := customSubject(ScopeOrg, []int{sales}, "key.freeze")
	require.True(t, Can(wide, "key.freeze", Target{DepartmentId: product}))
	narrow := customSubject(ScopeSelf, []int{sales}, "key.freeze")
	require.False(t, Can(narrow, "key.freeze", Target{DepartmentId: sales}))
}

// PRD §2, the staff row: 使用并查看分配给自己的 key，看自己的用量 — and that is
// true of every member, not of staff alone.
func TestCan_EveryMemberReadsTheirOwnKeysAndUsage(t *testing.T) {
	// A manager's own key sits in a department they do not manage.
	own := Target{DepartmentId: product, UserId: self}
	subjects := map[string]Subject{
		"finance": customSubject(ScopeOrg, nil, "usage.read"),
		"nothing": customSubject(ScopeDept, []int{sales}, "alert.read"),
	}
	for _, name := range []string{RoleOwner, RoleAdmin, RoleManager, RoleStaff, RoleReadonly} {
		subjects[name] = presetSubject(t, name)
	}
	for name, subject := range subjects {
		require.True(t, Can(subject, "key.read", own), "%s reads their own keys", name)
		require.True(t, Can(subject, "usage.read", own), "%s reads their own usage", name)
	}

	// Owning something is leave to look at it, never to change it.
	staff := presetSubject(t, RoleStaff)
	for _, primitive := range Primitives {
		if primitive == "key.read" || primitive == "usage.read" {
			continue
		}
		require.False(t, Can(staff, primitive, own), "%s on their own", primitive)
	}
	// A target that belongs to nobody is not "their own" for a subject that
	// carries no user id.
	require.False(t, Can(Subject{Scope: ScopeSelf}, "key.read", Target{}))
}

// Anything that is neither a primitive nor an inherent power is refused, for
// the owner too: a typo must fail closed.
func TestCan_RefusesWhatItDoesNotKnow(t *testing.T) {
	owner := presetSubject(t, RoleOwner)
	for _, action := range []string{"", "key", "key.raed", "key.*", "*", "wallet", "role.read", "department.read"} {
		require.False(t, Can(owner, action, Target{}), "%q", action)
		require.False(t, Can(owner, action, Target{DepartmentId: sales, UserId: self}), "%q", action)
	}
}

// List endpoints filter by Reach and single targets are judged by Can; the
// two must never disagree.
func TestReach_AgreesWithCan(t *testing.T) {
	subjects := []Subject{
		customSubject(ScopeOrg, nil, "key.assign", "usage.read"),
		customSubject(ScopeDept, []int{sales, support}, "key.assign", "usage.read"),
		customSubject(ScopeDept, nil, "key.assign"),
	}
	for _, name := range []string{RoleOwner, RoleAdmin, RoleManager, RoleStaff, RoleReadonly} {
		subjects = append(subjects, presetSubject(t, name))
	}
	for i, subject := range subjects {
		for _, primitive := range Primitives {
			everywhere, departments := Reach(subject, primitive)
			require.Equal(t, everywhere, Can(subject, primitive, Target{}), "subject %d, %s, whole organization", i, primitive)
			for _, department := range []int{sales, support, product} {
				inReach := everywhere
				for _, managed := range departments {
					inReach = inReach || managed == department
				}
				require.Equal(t, inReach, Can(subject, primitive, Target{DepartmentId: department, UserId: colleague}),
					"subject %d, %s, department %d", i, primitive, department)
			}
		}
	}
}

// PRD §2, the role pack table, written out by hand.
func TestRolePacks_MatchThePRD(t *testing.T) {
	want := map[string]struct {
		scope       string
		permissions string
	}{
		"it_ops":  {ScopeOrg, "key.read,key.create,key.update,key.assign,key.rotate,key.freeze,key.delete,member.read,usage.read,alert.read"},
		"hr_ops":  {ScopeOrg, "key.read,key.freeze,key.delete,member.read,member.invite,member.remove"},
		"finance": {ScopeOrg, "usage.read"},
	}
	require.Len(t, RolePacks, len(want))
	for _, pack := range RolePacks {
		expected, ok := want[pack.Key]
		require.True(t, ok, "unexpected role pack %s", pack.Key)
		require.Equal(t, expected.scope, pack.Scope, pack.Key)
		require.Equal(t, expected.permissions, JoinPermissions(pack.Permissions), pack.Key)
		// Stored as a custom role would store them: known, unique, in order.
		normalized, ok := NormalizePermissions(pack.Permissions)
		require.True(t, ok, pack.Key)
		require.Equal(t, pack.Permissions, normalized, pack.Key)
		require.NotEmpty(t, pack.Name, pack.Key)
		require.LessOrEqual(t, len([]rune(pack.Name)), 64, "%s fits org_roles.name", pack.Key)
		// A pack must be adoptable: its name can never be a preset's.
		for _, preset := range PresetRoles {
			require.NotEqual(t, preset.Name, pack.Name)
		}
	}
}

func TestNormalizePermissions(t *testing.T) {
	for _, tc := range []struct {
		requested []string
		want      []string
		ok        bool
	}{
		{[]string{"usage.read", "key.read"}, []string{"key.read", "usage.read"}, true},
		{[]string{"key.read", " key.read ", "key.read"}, []string{"key.read"}, true},
		{nil, []string{}, true},
		{Primitives, Primitives, true},
		{[]string{"key.read", "key.*"}, []string{"key.read"}, false},
		{[]string{PowerRoles}, []string{}, false},
		{[]string{""}, []string{}, false},
	} {
		got, ok := NormalizePermissions(tc.requested)
		require.Equal(t, tc.ok, ok, fmt.Sprint(tc.requested))
		require.Equal(t, tc.want, got, fmt.Sprint(tc.requested))
	}
}
