/*
Copyright (c) 2021 opensource@cmbc.com.cn
OpenGauss Operator is licensed under Mulan PSL v2.
You can use this software according to the terms and conditions of the Mulan PSL v2.
You may obtain a copy of Mulan PSL v2 at:
         http://license.coscl.org.cn/MulanPSL2
THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
See the Mulan PSL v2 for more details.
*/
package utils

import (
	"fmt"
	"strings"
)

type DBState struct {
	//0:否  1：是
	//opengauss进程是否存在
	ProcessExist  int32  `json:"chkprocess,omitempty"`
	ConnAvailable int32  `json:"chkconn,omitempty"`
	Maintenance   int32  `json:"maintenance,omitempty"`
	Pending       int32  `json:"pending,omitempty"`
	Primary       int32  `json:"primary,omitempty"`
	Standby       int32  `json:"standby,omitempty"`
	Standalone    int32  `json:"standalone,omitempty"`
	Hang          int32  `json:"hang,omitempty"`
	Normal        int32  `json:"chkrepl,omitempty"`
	BuildStatus   int32  `json:"buildstatus,omitempty"`
	BackupStatus  int32  `json:"basebackup,omitempty"`
	RestoreStatus int32  `json:"restore,omitempty"`
	Connections   int32  `json:"connections,omitempty"`
	DetailInfo    string `json:"detailinfo,omitempty"`
}

var initDBState = DBState{
	ProcessExist:  0,
	ConnAvailable: 0,
	Maintenance:   0,
	Pending:       0,
	Primary:       0,
	Standby:       0,
	Standalone:    0,
	Hang:          0,
	Normal:        0,
	BuildStatus:   3, //初始值设置为3，表示尚未开始build， gs_ctl querybuild -D ${PGDATA} 查询结果为最后一次build状态
	BackupStatus:  0,
	RestoreStatus: 0,
	Connections:   0,
	DetailInfo:    "",
}

func InitDBState() DBState {
	return initDBState
}

func (state DBState) String() string {
	str := "[Primary: %t, Standby: %t, Pending: %t, Standalone: %t, Process Exist: %t, Connection available: %t, Maintenance: %t, Normal: %t, Build status: %t, Backup status: %t, Restore status: %t, Hang: %t, Connections: %d, Detail: %s]"
	return fmt.Sprintf(str, state.IsPrimary(), state.IsStandby(), state.IsPending(), state.IsStandalone(), state.IsProcessExist(), state.IsConnectionAvailble(), state.IsInMaintenance(), state.IsNormal(), state.IsBuildComplete(), state.IsBackupComplete(), state.IsRestoreComplete(), state.IsHang(), state.Connections, state.DetailInfo)
}

func (state DBState) PrintableString() string {
	var stateString strings.Builder
	stateString.WriteString("[")
	if state.IsPrimary() {
		stateString.WriteString("Local Role: Primary, ")
	} else if state.IsStandby() {
		stateString.WriteString("Local Role: Standby, ")
	} else if state.IsPending() {
		stateString.WriteString("Local Role: Pending, ")
	} else if state.IsStandalone() {
		stateString.WriteString("Local Role: Standalone, ")
	} else {
		stateString.WriteString("Local Role: None, ")
	}
	stateString.WriteString(fmt.Sprintf("Process exist: %t, ", state.IsProcessExist()))
	stateString.WriteString(fmt.Sprintf("Connection available: %t, ", state.IsConnectionAvailble()))
	stateString.WriteString(fmt.Sprintf("DB state normal: %t, ", state.IsNormal()))
	stateString.WriteString(fmt.Sprintf("Maintenance: %t, ", state.IsInMaintenance()))
	stateString.WriteString(fmt.Sprintf("Build status: %s, ", state.GetBuildStatus()))
	backupStatus := ""
	switch state.BackupStatus {
	case 1:
		backupStatus = "complete"
	case 2:
		backupStatus = "failed"
	case 3:
		backupStatus = "in process"
	default:
		backupStatus = "no data"
	}
	stateString.WriteString(fmt.Sprintf("Backup status: %s, ", backupStatus))

	restoreStatus := ""
	switch state.RestoreStatus {
	case 1:
		restoreStatus = "complete"
	case 2:
		restoreStatus = "failed"
	case 3:
		restoreStatus = "in process"
	default:
		restoreStatus = "no data"
	}
	stateString.WriteString(fmt.Sprintf("Restore status: %s, ", restoreStatus))
	stateString.WriteString(fmt.Sprintf("Static Connections: %d, ", state.Connections))
	if state.DetailInfo != "" {
		stateString.WriteString(fmt.Sprintf("Detail Information: %s, ", state.DetailInfo))
	}
	stateString.WriteString("]")
	return stateString.String()
}

func (state DBState) Equals(another DBState) bool {
	if state.ProcessExist != another.ProcessExist {
		return false
	}
	if state.ConnAvailable != another.ConnAvailable {
		return false
	}
	if state.Maintenance != another.Maintenance {
		return false
	}
	if state.Pending != another.Pending {
		return false
	}
	if state.Primary != another.Primary {
		return false
	}
	if state.Standby != another.Standby {
		return false
	}
	if state.Standalone != another.Standalone {
		return false
	}
	if state.Hang != another.Hang {
		return false
	}
	if state.Normal != another.Normal {
		return false
	}
	if state.BuildStatus != another.BuildStatus {
		return false
	}
	if state.BackupStatus != another.BackupStatus {
		return false
	}
	if state.RestoreStatus != another.RestoreStatus {
		return false
	}
	if state.DetailInfo != another.DetailInfo {
		return false
	}
	return true
}

func (state DBState) NeedConfigure() bool {
	if !state.IsProcessExist() {
		return true
	}
	if !state.IsConnectionAvailble() {
		return true
	}
	if !state.IsNormal() {
		return true
	}
	if state.IsPending() {
		return true
	}
	if state.IsStandalone() {
		return true
	}
	if state.IsHang() {
		return true
	}
	return false
}

/*
判断opengauss进程是否存在
返回值：
    true:存在
	false: 不存在
*/
func (state DBState) IsProcessExist() bool {
	return state.ProcessExist == 1
}

func (state DBState) IsConnectionAvailble() bool {
	return state.ConnAvailable == 1
}

func (state DBState) IsInMaintenance() bool {
	return state.Maintenance == 1
}

func (state DBState) IsPending() bool {
	return state.Pending == 1
}

func (state DBState) IsPrimary() bool {
	return state.Primary == 1
}

func (state DBState) IsStandby() bool {
	return state.Standby == 1
}

func (state DBState) IsStandalone() bool {
	return state.Standalone == 1
}

func (state DBState) IsHang() bool {
	return state.Hang == 1
}

func (state DBState) IsNormal() bool {
	return state.Normal == 1
}

func (state DBState) IsBuildComplete() bool {
	return state.BuildStatus == 0
}

func (state DBState) IsBuilding() bool {
	return state.BuildStatus == 2
}

func (state DBState) IsBuildFail() bool {
	return state.BuildStatus == 1
}

func (state DBState) BackupStarted() bool {
	return state.BackupStatus != 0
}

func (state DBState) IsBackupComplete() bool {
	return state.BackupStatus == 1
}

func (state DBState) IsBackupFailed() bool {
	return state.BackupStatus == 2
}

func (state DBState) IsBackupInProcess() bool {
	return state.BackupStatus == 3
}

func (state DBState) RestoreStarted() bool {
	return state.RestoreStatus != 0
}

func (state DBState) IsRestoreComplete() bool {
	return state.RestoreStatus == 1
}

func (state DBState) IsRestoreFailed() bool {
	return state.RestoreStatus == 2
}

func (state DBState) IsRestoreProcess() bool {
	return state.RestoreStatus == 3
}

func (state DBState) IsConfigured() bool {
	return state.Connections > 0
}

func (state DBState) IsDisconnected() bool {
	return !state.IsNormal() && (state.DetailInfo == "Disconnected" || state.DetailInfo == "Connecting...")
}
func (state DBState) GetBuildStatus() string {
	buildStatus := ""
	switch state.BuildStatus {
	case 0:
		buildStatus = "Build completed"
	case 1:
		buildStatus = "Build failed"
	case 2:
		buildStatus = "Building"
	case 3:
		buildStatus = "no begin"
	default:
		buildStatus = "unknown"
	}
	return buildStatus
}
