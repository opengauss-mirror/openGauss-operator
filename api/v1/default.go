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

package v1

import (
	"opengauss-operator/utils"
)

const (
	CRD_KIND                       = "OpenGaussCluster"
	DEFAULT_CPU_LIMIT              = "1"
	DEFAULT_MEMORY_LIMIT           = "4Gi"
	DEFAULT_SIDECAR_CPU_LIMIT      = "500m"
	DEFAULT_SIDECAR_MEMORY_LIMIT   = "1Gi"
	DEFAULT_BACKUP_PATH            = "/data/k8slocalxbk"
	DEFAULT_ARCHIVE_PATH           = "/data/k8slocalogarchive"
	LOCAL_ROLE_PRIMARY             = "primary"
	LOCAL_ROLE_STANDBY             = "standby"
	DB_CPU_REQ                     = "500m"
	DB_MEM_REQ                     = "2Gi"
	DB_STORAGE_REQ                 = "3Gi"
	SIDECAR_CPU_REQ                = "200m"
	SIDECAR_MEM_REQ                = "500Mi"
	SIDECAR_STORAGE_REQ            = "1Gi"
	DEFAULT_STORAGE_CLASS          = "topolvm-provisioner"
	DEFAULT_DB_PORT                = 5432
	DEFAULT_DB_PASSWD              = "SzhTQGFkbWlu"
	DEFAULT_SCRIPT_CM_NAME         = "opengauss-script-config"
	DEFAULT_FILEBEAT_CM_NAME       = "opengauss-filebeat-config"
	DEFAULT_PROCESS_TIMEOUT        = 300
	DEFAULT_GRACE_PERIOD           = 30
	DEFAULT_TOLERATION             = 300
	DEFAULT_MOST_AVAILABLE_TIMEOUT = 60
	OG_MONITOR_TYPE_KEY            = "og_MONITOR_TYPE"
	OG_MONITOR_TYPE_VAL            = "OPENGAUSS"
	NETWORK_CALICO                 = "calico"
	NETWORK_KUBE_OVN               = "kube-ovn"
	DEFAULT_POLLING_PERIOD         = 60
	DEFAULT_LIVENESS_PROBE_PERIOD  = 30
	DEFAULT_READINESS_PROBE_PERIOD = 30
)

/*
设置默认值
*/
func (in *OpenGaussCluster) DefaultSpec() bool {
	update := false
	if in.Spec.LocalRole == "" {
		in.Spec.LocalRole = LOCAL_ROLE_PRIMARY
		update = true
	}
	if in.Spec.DBPort == 0 {
		in.Spec.DBPort = DEFAULT_DB_PORT
		update = true
	}

	if in.Spec.DBPasswd == "" {
		in.Spec.DBPasswd = DEFAULT_DB_PASSWD
		update = true
	}

	if in.Spec.Cpu == "" {
		in.Spec.Cpu = DEFAULT_CPU_LIMIT
		update = true
	}

	if in.Spec.Memory == "" {
		in.Spec.Memory = DEFAULT_MEMORY_LIMIT
		update = true
	}

	//PVC resource的值不能减小，为避免错误，对非新建集群的存储，默认使用已有数据
	//其他storage属性也采用相同逻辑
	if in.Spec.Storage == "" {
		if !in.IsNew() && in.Status.Spec.Storage != "" {
			in.Spec.Storage = in.Status.Spec.Storage
		} else {
			in.Spec.Storage = DB_STORAGE_REQ
		}
		update = true
	}

	if in.Spec.SidecarCpu == "" {
		in.Spec.SidecarCpu = DEFAULT_SIDECAR_CPU_LIMIT
		update = true
	}

	if in.Spec.SidecarMemory == "" {
		in.Spec.SidecarMemory = DEFAULT_SIDECAR_MEMORY_LIMIT
		update = true
	}

	if in.Spec.SidecarStorage == "" {
		if !in.IsNew() && in.Status.Spec.SidecarStorage != "" {
			in.Spec.SidecarStorage = in.Status.Spec.SidecarStorage
		} else {
			in.Spec.SidecarStorage = SIDECAR_STORAGE_REQ
		}
		update = true
	}

	if in.Spec.BackupPath == "" {
		in.Spec.BackupPath = DEFAULT_BACKUP_PATH
		update = true
	}

	if in.Spec.ArchiveLogPath == "" {
		in.Spec.ArchiveLogPath = DEFAULT_ARCHIVE_PATH
		update = true
	}
	if in.Spec.HostpathRoot == "" && in.Spec.StorageClass == "" {
		in.Spec.StorageClass = DEFAULT_STORAGE_CLASS
		update = true
	}
	if in.Spec.ScriptConfig == "" {
		in.Spec.ScriptConfig = DEFAULT_SCRIPT_CM_NAME
		update = true
	}
	if in.Spec.FilebeatConfig == "" {
		in.Spec.FilebeatConfig = DEFAULT_FILEBEAT_CM_NAME
		update = true
	}
	if sc, updated := updateScheduleConfig(in.Spec.Schedule); updated {
		in.Spec.Schedule = sc
		update = true
	}
	if in.Spec.Config == nil || len(in.Spec.Config) == 0 {
		in.Spec.Config = make(map[string]string)
	} else {
		mergedConfig := utils.MergeMaps(in.Status.Spec.Config, in.Spec.Config)
		if !utils.CompareMaps(in.Spec.Config, mergedConfig) {
			in.Spec.Config = mergedConfig
			update = true
		}
	}
	if len(in.Spec.CustomizedEnv) == 0 {
		internalMap := make(map[string]string)
		internalMap[OG_MONITOR_TYPE_KEY] = OG_MONITOR_TYPE_VAL
		in.Spec.CustomizedEnv = internalMap
		update = true
	}
	//如果是存量集群，且NetworkClass属性为空，则设置NetworkClass为calico，即兼容旧版operator，已经部署的og集群
	if !in.IsNew() && in.Spec.NetworkClass == "" {
		in.Spec.NetworkClass = NETWORK_CALICO
		update = true
	}
	return update
}

func updateScheduleConfig(sc ScheduleConfig) (ScheduleConfig, bool) {
	update := false
	if sc.ProcessTimeout == 0 {
		sc.ProcessTimeout = DEFAULT_PROCESS_TIMEOUT
		update = true
	}
	if sc.GracePeriod == 0 {
		sc.GracePeriod = DEFAULT_GRACE_PERIOD
		update = true
	}
	if sc.Toleration == 0 {
		sc.Toleration = DEFAULT_TOLERATION
		update = true
	}
	if sc.MostAvailableTimeout == 0 {
		sc.MostAvailableTimeout = DEFAULT_MOST_AVAILABLE_TIMEOUT
		update = true
	}
	if sc.PollingPeriod == 0 {
		sc.PollingPeriod = DEFAULT_POLLING_PERIOD
		update = true
	}
	if sc.LivenessProbePeriod == 0 {
		sc.LivenessProbePeriod = DEFAULT_LIVENESS_PROBE_PERIOD
		update = true
	}
	if sc.ReadinessProbePeriod == 0 {
		sc.ReadinessProbePeriod = DEFAULT_READINESS_PROBE_PERIOD
		update = true
	}
	return sc, update
}
