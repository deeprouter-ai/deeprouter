package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

// defaultPlatformGroup is the group every account starts in (users.group's
// column default). Only accounts still there are moved: one an admin has put
// in another group by hand is left alone.
const defaultPlatformGroup = "default"

// memberGroupBatch caps the ids in one UPDATE ... WHERE id IN (...), well
// under every engine's limit on bound parameters.
const memberGroupBatch = 500

// MemberGroupSetting is which platform group an organization's accounts go
// into. A platform admin turns it on; nobody inside an organization can.
//
// It is two settings rather than one so the code can ship before the group is
// ready (PRD D49): the group name already says `enterprise`, and nothing moves
// until Enabled is switched on — which waits until that group has channels.
// Stored in the options table as org_setting.member_group and
// org_setting.member_group_enabled; deploy/settings.env can set them.
type MemberGroupSetting struct {
	Group   string `json:"member_group"`
	Enabled bool   `json:"member_group_enabled"`
}

var memberGroupSetting = MemberGroupSetting{
	Group:   "enterprise",
	Enabled: false,
}

func init() {
	config.GlobalConfig.Register("org_setting", &memberGroupSetting)
}

// GetMemberGroupSetting returns the setting as it stands now.
func GetMemberGroupSetting() MemberGroupSetting {
	return memberGroupSetting
}

// memberGroupToAssign names the group an organization account should be put
// in, or "" when it should be left where it is: the switch is off, the name is
// empty, or no group of that name exists in the group ratios. An unknown name
// is refused rather than written, because an account in a group with no ratio
// would be priced and routed against nothing.
func memberGroupToAssign() string {
	current := GetMemberGroupSetting()
	if !current.Enabled {
		return ""
	}
	group := strings.TrimSpace(current.Group)
	if group == "" || !ratio_setting.ContainsGroupRatio(group) {
		return ""
	}
	return group
}

// withMemberGroup adds the platform group to the columns an entry point writes
// when an account joins an organization, if one is to be assigned. The map is
// returned so the call reads in place: Updates(withMemberGroup(map[...]{...})).
// "group" is a reserved word; GORM quotes map keys in SET on every engine.
func withMemberGroup(columns map[string]any) map[string]any {
	if group := memberGroupToAssign(); group != "" {
		columns["group"] = group
	}
	return columns
}

// ApplyMemberGroup moves the organization accounts that are still in the
// default group into the member group, and reports how many it moved. It is
// what turns the switch on for the accounts that existed before it: the entry
// points only see accounts joining from then on. A background task calls it
// on a timer (service/org_member_group.go), so it must be cheap and harmless
// to repeat — with nothing left to move it changes nothing.
//
// The configuration manager has no change hook and the org migration runs
// before the options are loaded, so there is no single moment to migrate at;
// a repeated pass also catches any account an entry point ever misses.
//
// Personal accounts (org_id = 0) are never touched, nor are removed members
// (soft-deleted rows are outside the model's default scope). A failure to drop
// an account from the user cache is logged, not returned: the row is already
// right, and the cached copy expires on its own.
func ApplyMemberGroup(db *gorm.DB) (int, error) {
	group := memberGroupToAssign()
	if group == "" || group == defaultPlatformGroup {
		return 0, nil
	}
	var ids []int
	err := db.Model(&platformmodel.User{}).
		Where("org_id <> ?", 0).
		Where(map[string]any{"group": defaultPlatformGroup}).
		Pluck("id", &ids).Error
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	moved := 0
	for start := 0; start < len(ids); start += memberGroupBatch {
		batch := ids[start:min(start+memberGroupBatch, len(ids))]
		// The group condition is repeated so an account an admin moved
		// between the read and the write keeps the group it was given.
		result := db.Model(&platformmodel.User{}).
			Where("id IN ?", batch).
			Where(map[string]any{"group": defaultPlatformGroup}).
			Update("group", group)
		if result.Error != nil {
			return moved, result.Error
		}
		moved += int(result.RowsAffected)
		for _, id := range batch {
			if err := platformmodel.InvalidateUserCache(id); err != nil {
				common.SysError("org: failed to drop an account from the user cache after moving it to the member group: " + err.Error())
			}
		}
	}
	return moved, nil
}
