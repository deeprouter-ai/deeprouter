package ratio_setting

import (
	"encoding/json"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

// defaultGroupRatio — DeepRouter pricing ladder (see docs/PRD.md §7).
//
//   ratio  = sell_price / upstream_cost
//   markup = ratio - 1                 (e.g. 1.2 = +20%)
//   margin = (sell - cost) / sell      (e.g. ratio 1.2 → ~17% margin)
//
// Default is a CONSERVATIVE +20% markup across the board.
//
// History / rationale:
//   - v0.1 first ladder was aggressive (default 3.333× / enterprise 4.0×
//     targeting 70-75% gross margin). Operator (Lightman, 2026-05-16)
//     flagged that as too high for a launch default — if a ratio or
//     upstream price is mis-configured the gateway over-charges customers
//     before we notice. Lost a customer > made an extra dollar.
//   - Revised default to flat 1.2× so a fresh install is safe.
//     Operators move specific groups up to 2.0-3.5× later, deliberately,
//     per business decision — not via the default.
//
// Operators override per group at runtime via admin UI System Settings →
// Operations → 分组倍率, or PUT /api/option key=GroupRatio. Existing
// deployments need an explicit PUT — this default only affects fresh
// installs (AddAll only fills missing keys).
//
// Group names align with PLAN.md / AIRBOTIX.md "Tenants (V0)" table —
// airbotix-kids / jr-academy are the two launch tenants; default / vip /
// svip / enterprise are the external SaaS tiers.
var defaultGroupRatio = map[string]float64{
	"default":       1.2, // safe baseline — +20% markup (~17% margin)
	"vip":           1.2, // bump up deliberately when running real VIP promos
	"svip":          1.2,
	"enterprise":    1.2, // raise to 2.0-3.5× per enterprise contract
	"airbotix-kids": 1.2, // own product — small buffer for infra overhead
	"jr-academy":    1.2, // own product — same
}

var groupRatioMap = types.NewRWMap[string, float64]()

// defaultGroupGroupRatio — per (user_group × channel_group) cross-table.
// Empty out-of-the-box; populate via admin UI when you want patterns like
// "VIP customer × premium-channel-pool gets 10% off".
var defaultGroupGroupRatio = map[string]map[string]float64{}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{
	"vip": {
		"append_1":   "vip_special_group_1",
		"-:remove_1": "vip_removed_group_1",
	},
}

type GroupRatioSetting struct {
	GroupRatio              *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio         *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupSpecialUsableGroup *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup: groupSpecialUsableGroup,
		GroupRatio:              groupRatioMap,
		GroupGroupRatio:         groupGroupRatioMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}
