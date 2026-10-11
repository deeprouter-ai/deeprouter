package service

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

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
