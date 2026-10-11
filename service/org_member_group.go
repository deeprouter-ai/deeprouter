package service

// Enterprise Org member group (meta-repo docs/enterprise-org-prd.md D49): the
// background task that keeps organization accounts out of the default group
// once org_setting.member_group is switched on. What it does is decided in
// internal/org/service (member_group.go); this file holds the loop.

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

// orgMemberGroupTick is how often the task looks. Switching the setting on
// moves existing members within one tick; the entry points cover everyone who
// joins after that, so the tick only bounds how long the backlog takes.
const orgMemberGroupTick = 5 * time.Minute

// orgMemberGroupOnce makes StartOrgMemberGroupTask start the task a single time.
var orgMemberGroupOnce sync.Once

// StartOrgMemberGroupTask starts the task that moves organization accounts
// still in the default group into the member group. It runs on the master
// node only, like the platform's other periodic tasks, and looks once right
// away. With the setting off — the default — every pass returns at once.
func StartOrgMemberGroupTask() {
	orgMemberGroupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			ticker := time.NewTicker(orgMemberGroupTick)
			defer ticker.Stop()
			applyOrgMemberGroup()
			for range ticker.C {
				applyOrgMemberGroup()
			}
		})
	})
}

// applyOrgMemberGroup is one pass. A pass that fails is reported and tried
// again on the next tick; it never takes the task down.
func applyOrgMemberGroup() {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("organization member group task: panic: %v", r))
		}
	}()
	moved, err := orgservice.ApplyMemberGroup(model.DB)
	if err != nil {
		common.SysError("organization member group task: " + err.Error())
	}
	if moved > 0 {
		common.SysLog(fmt.Sprintf("organization member group task: moved %d organization account(s) into group %q",
			moved, orgservice.GetMemberGroupSetting().Group))
	}
}
