package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The org columns on the three platform tables migrate with the core
// AutoMigrate in model/main.go, where a failure stops the gateway from
// booting. These tests run that step on every engine, both on empty tables
// and on tables that already hold rows — which is what a deploy does.

// legacyUser, legacyToken and legacyLog are the three tables as they looked
// before Enterprise Org, cut down to what the migration has to cope with: a
// populated table with none of the new columns. The unique columns are kept
// because every real installation already has them, and SQLite cannot add a
// UNIQUE column to an existing table.
type legacyUser struct {
	Id          int     `gorm:"primaryKey"`
	Username    string  `gorm:"unique;index"`
	Password    string  `gorm:"not null"`
	Role        int     `gorm:"type:int;default:1"`
	AccessToken *string `gorm:"type:char(32);column:access_token;uniqueIndex"`
	AffCode     string  `gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
}

// TableName points the legacy shape at the real table.
func (legacyUser) TableName() string { return "users" }

type legacyToken struct {
	Id     int    `gorm:"primaryKey"`
	UserId int    `gorm:"index"`
	Key    string `gorm:"type:varchar(128);uniqueIndex"`
	Name   string `gorm:"index"`
}

// TableName points the legacy shape at the real table.
func (legacyToken) TableName() string { return "tokens" }

type legacyLog struct {
	Id        int   `gorm:"primaryKey"`
	UserId    int   `gorm:"index"`
	CreatedAt int64 `gorm:"bigint"`
	Type      int
	Content   string
}

// TableName points the legacy shape at the real table.
func (legacyLog) TableName() string { return "logs" }

// migratePlatformTables runs the same AutoMigrate call as model/main.go for
// the three tables that carry org columns.
func migratePlatformTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&platformmodel.Token{}, &platformmodel.User{}, &platformmodel.Log{}))
}

func TestPlatformTables_CarryTheOrgColumnsAndIndexes(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		migratePlatformTables(t, db)
		for table, columns := range map[string][]string{
			"users":  {"org_id", "role_id", "department_id", "is_service"},
			"tokens": {"org_id", "created_by", "policy_template"},
			"logs":   {"org_id", "department_id"},
		} {
			for _, column := range columns {
				require.True(t, db.Migrator().HasColumn(table, column), "column %s.%s", table, column)
			}
		}
		// PRD §7.2 asks for these four to be indexed: reports filter on them.
		require.True(t, db.Migrator().HasIndex(&platformmodel.User{}, "OrgId"), "index on users.org_id")
		require.True(t, db.Migrator().HasIndex(&platformmodel.Token{}, "OrgId"), "index on tokens.org_id")
		require.True(t, db.Migrator().HasIndex(&platformmodel.Log{}, "OrgId"), "index on logs.org_id")
		require.True(t, db.Migrator().HasIndex(&platformmodel.Log{}, "DepartmentId"), "index on logs.department_id")
	})
}

// The regression baseline of the whole feature: every account, key and log
// line that existed before the upgrade must come out of it as personal —
// org_id 0 and nothing else set — and must still load.
func TestPlatformTables_ExistingRowsStayPersonalAfterTheUpgrade(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, db.AutoMigrate(&legacyUser{}, &legacyToken{}, &legacyLog{}))
		require.NoError(t, db.Create(&[]legacyUser{
			{Username: "alice", Password: "hash-a", Role: 1, AffCode: "aff1"},
			{Username: "root", Password: "hash-r", Role: 100, AffCode: "aff2"},
		}).Error)
		require.NoError(t, db.Create(&[]legacyToken{
			{UserId: 1, Key: "legacy-key-1", Name: "first"},
			{UserId: 1, Key: "legacy-key-2", Name: "second"},
		}).Error)
		require.NoError(t, db.Create(&[]legacyLog{
			{UserId: 1, CreatedAt: 1700000000, Type: 2, Content: "consumed"},
			{UserId: 2, CreatedAt: 1700000001, Type: 1, Content: "topped up"},
		}).Error)

		migratePlatformTables(t, db)
		// A second boot must be a no-op.
		migratePlatformTables(t, db)

		var users []platformmodel.User
		require.NoError(t, db.Order("id").Find(&users).Error)
		require.Len(t, users, 2)
		for _, user := range users {
			require.Zero(t, user.OrgId, user.Username)
			require.Zero(t, user.OrgRoleId, user.Username)
			require.Zero(t, user.DepartmentId, user.Username)
			require.False(t, user.IsService, user.Username)
		}
		// The global role is untouched by the new columns.
		require.Equal(t, 1, users[0].Role)
		require.Equal(t, 100, users[1].Role)

		var tokens []platformmodel.Token
		require.NoError(t, db.Order("id").Find(&tokens).Error)
		require.Len(t, tokens, 2)
		for _, token := range tokens {
			require.Zero(t, token.OrgId, token.Name)
			require.Zero(t, token.CreatedBy, token.Name)
			require.Empty(t, token.PolicyTemplate, token.Name)
		}

		var logs []platformmodel.Log
		require.NoError(t, db.Order("id").Find(&logs).Error)
		require.Len(t, logs, 2)
		for _, log := range logs {
			require.Zero(t, log.OrgId, log.Content)
			require.Zero(t, log.DepartmentId, log.Content)
		}
	})
}

// Several endpoints answer with whole rows (the key list, the usage log). A
// personal row must serialize exactly as it did before the org columns
// existed, so none of the new fields may appear while they are zero.
func TestPlatformRows_PersonalJSONHasNoOrgFields(t *testing.T) {
	orgFields := []string{"org_id", "org_role_id", "department_id", "is_service", "created_by", "policy_template"}
	for name, row := range map[string]any{
		"user":  platformmodel.User{Id: 1, Username: "solo", Role: common.RoleCommonUser},
		"token": platformmodel.Token{Id: 1, UserId: 1, Name: "personal"},
		"log":   platformmodel.Log{Id: 1, UserId: 1, Content: "consumed"},
	} {
		encoded, err := common.Marshal(row)
		require.NoError(t, err)
		for _, field := range orgFields {
			require.NotContains(t, string(encoded), `"`+field+`"`, "%s row", name)
		}
	}

	// ...and they do appear once a row belongs to an organization.
	encoded, err := common.Marshal(platformmodel.Token{Id: 2, OrgId: 3, CreatedBy: 4, PolicyTemplate: "coding"})
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"org_id":3`)
	require.Contains(t, string(encoded), `"created_by":4`)
	require.Contains(t, string(encoded), `"policy_template":"coding"`)
}
