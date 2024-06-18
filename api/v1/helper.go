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
	"fmt"
	"opengauss-operator/utils"
	"reflect"
	"strings"
)

const (
	READ_SVC_NAME  = "og-%s-svc-read"
	WRITE_SVC_NAME = "og-%s-svc"
)

func (in *OpenGaussCluster) IsPrimary() bool {
	localRole := in.GetValidSpec().LocalRole
	return localRole == "" || localRole == LOCAL_ROLE_PRIMARY
}

func (in *OpenGaussCluster) IsStandby() bool {
	localRole := in.GetValidSpec().LocalRole
	return localRole != "" && localRole == LOCAL_ROLE_STANDBY
}

func (in *OpenGaussCluster) IsNew() bool {
	return in.Status.Spec.Image == ""
}

//CR是否变更，检查项包括资源（CPU、内存、镜像、存储、端口、节点）、备库节点、数据库参数、维护状态
func (in *OpenGaussCluster) IsChanged() bool {
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return !currSpec.Equals(lastSpec)
}

func (in *OpenGaussCluster) IsResourceChange() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.resourceChange(lastSpec)
}

func (in *OpenGaussCluster) IsUpgrade() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.containerChange(lastSpec)
}

func (in *OpenGaussCluster) IsStorageExpansion() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.storageChange(lastSpec)
}

func (in *OpenGaussCluster) IsServiceChange() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.serviceChange(lastSpec)
}

func (in *OpenGaussCluster) IsRoleChange() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.roleChange(lastSpec)
}

func (in *OpenGaussCluster) IsDBConfigChange() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.dbConfigChange(lastSpec)
}

func (in *OpenGaussCluster) IsRestoreChange() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.restoreChange(lastSpec)
}
func (in *OpenGaussCluster) RestoreRequired() bool {
	return in.GetValidSpec().RestoreFile != "" && in.Status.RestorePhase != RestorePhaseSucceeded
}

func (in *OpenGaussCluster) RestoreComplete() bool {
	return in.IsRestoreChange() && in.Status.RestorePhase == RestorePhaseSucceeded
}

/*
集群的 pod主ip是否改变
*/
func (in *OpenGaussCluster) IsIpListChange() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.ipListChange(lastSpec)
}
func (in *OpenGaussCluster) IsRemoteIpListChange() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.remoteIpListChange(lastSpec)
}

func (in *OpenGaussCluster) IsHostpathEnable() bool {
	return in.GetValidSpec().HostpathRoot != ""
}

func (in *OpenGaussCluster) IsValid() bool {
	return in.Status.State != OpenGaussClusterStateInvalid
}
func (in *OpenGaussCluster) IsMaintainStart() bool {
	return in.Spec.isInMaintain() && !in.Status.Spec.isInMaintain()
}
func (in *OpenGaussCluster) IsMaintainEnd() bool {
	return !in.Spec.isInMaintain() && in.Status.Spec.isInMaintain()
}
func (in *OpenGaussCluster) IsMaintaining() bool {
	return in.Spec.isInMaintain() && in.Status.Spec.isInMaintain()
}
func (in *OpenGaussCluster) MaintainModeChange() bool {
	if in.IsNew() {
		return false
	}
	lastSpec := in.Status.Spec.DeepCopy()
	currSpec := in.GetValidSpec()
	return currSpec.Maintenance != lastSpec.Maintenance
}

func (in *OpenGaussCluster) GetValidSpec() *OpenGaussClusterSpec {
	spec := in.Spec
	if !in.IsValid() {
		spec = in.Status.Spec
	}
	return &spec
}

func (in *OpenGaussClusterSpec) dbConfigChange(another *OpenGaussClusterSpec) bool {
	return !utils.CompareMaps(in.Config, another.Config)
}

func (in *OpenGaussClusterSpec) remoteIpListChange(another *OpenGaussClusterSpec) bool {
	if len(in.RemoteIpList) == 0 && len(another.RemoteIpList) == 0 {
		return false
	}
	if len(in.RemoteIpList) != len(another.RemoteIpList) {
		return true
	}
	thisSet := in.RemoteIpSet()
	thatSet := another.RemoteIpSet()
	return !thisSet.Equals(thatSet)
}

func (in *OpenGaussClusterSpec) storageChange(another *OpenGaussClusterSpec) bool {
	if in.Storage != another.Storage {
		return true
	}
	if in.SidecarStorage != another.SidecarStorage {
		return true
	}
	return false
}

func (in *OpenGaussClusterSpec) serviceChange(another *OpenGaussClusterSpec) bool {
	if in.ReadPort != another.ReadPort {
		return true
	}
	if in.WritePort != another.WritePort {
		return true
	}
	return false
}

/*
判断cluster的Image，SidecarImage，Cpu,Memory，BandWidth，SidecarCpu，SidecarMemory，BackupPath，ArchiveLogPath
ScriptConfig，FilebeatConfig，CustomizedEnv是否发生变化
*/
func (in *OpenGaussClusterSpec) containerChange(another *OpenGaussClusterSpec) bool {
	if in.Image != another.Image {
		return true
	}
	if in.SidecarImage != another.SidecarImage {
		return true
	}
	if in.Cpu != another.Cpu {
		return true
	}
	if in.Memory != another.Memory {
		return true
	}
	if in.BandWidth != another.BandWidth {
		return true
	}
	if in.SidecarCpu != another.SidecarCpu {
		return true
	}
	if in.SidecarMemory != another.SidecarMemory {
		return true
	}
	if in.BackupPath != another.BackupPath {
		return true
	}
	if in.ArchiveLogPath != another.ArchiveLogPath {
		return true
	}
	if in.ScriptConfig != another.ScriptConfig {
		return true
	}
	if in.FilebeatConfig != another.FilebeatConfig {
		return true
	}
	// 比较自定义环境变量是否发生变化
	if !utils.CompareMaps(in.CustomizedEnv, another.CustomizedEnv) {
		return true
	}
	// 比较liveness和readiness探针周期是否改变
	if in.Schedule.LivenessProbePeriod != another.Schedule.LivenessProbePeriod {
		return true
	}
	if in.Schedule.ReadinessProbePeriod != another.Schedule.ReadinessProbePeriod {
		return true
	}
	return false
}

func (in *OpenGaussClusterSpec) roleChange(another *OpenGaussClusterSpec) bool {
	return in.LocalRole != another.LocalRole
}

/*
校验资源是否发生变化 ，包括container，storage，service，iplist，
*/
func (in *OpenGaussClusterSpec) resourceChange(another *OpenGaussClusterSpec) bool {
	if in.containerChange(another) {
		return true
	}
	if in.storageChange(another) {
		return true
	}
	if in.serviceChange(another) {
		return true
	}
	if in.ipNodeListChange(another) {
		return true
	}
	if in.annotationChange(another) {
		return true
	}
	if in.labelChange(another) {
		return true
	}
	return false
}

/*
集群的 pod主ip是否改变
*/
func (in *OpenGaussClusterSpec) ipListChange(another *OpenGaussClusterSpec) bool {
	if len(in.IpList) != len(another.IpList) {
		return true
	} else {
		specIPSet := in.IpSet()
		anotherIPSet := another.IpSet()
		if !specIPSet.Equals(anotherIPSet) {
			return true
		}
	}
	return false
}

/*
集群的 IpNodeEntry是否改变，包括Ip,NodeName和ExtendIp
*/
func (in *OpenGaussClusterSpec) ipNodeListChange(another *OpenGaussClusterSpec) bool {
	if len(in.IpList) != len(another.IpList) {
		return true
	} else {
		specIpEntryMap := make(map[string]IpNodeEntry, 0)
		anotherIpEntryMap := make(map[string]IpNodeEntry, 0)
		for _, ipNode := range in.IpList {
			specIpEntryMap[ipNode.Ip] = ipNode
		}
		for _, ipNode := range another.IpList {
			anotherIpEntryMap[ipNode.Ip] = ipNode
		}
		return !reflect.DeepEqual(specIpEntryMap, anotherIpEntryMap)
	}
}

func (in *OpenGaussClusterSpec) Equals(another *OpenGaussClusterSpec) bool {
	if in.resourceChange(another) {
		return false
	}
	if in.dbConfigChange(another) {
		return false
	}
	if in.remoteIpListChange(another) {
		return false
	}
	if in.roleChange(another) {
		return false
	}
	if in.restoreChange(another) {
		return false
	}
	if !in.Schedule.Equals(another.Schedule) {
		return false
	}
	return true
}
func (in *OpenGaussClusterSpec) isInMaintain() bool {
	return in.Maintenance
}

func (in *OpenGaussClusterSpec) IpSet() utils.Set {
	ipSet := utils.NewSet()
	for _, entry := range in.IpList {
		ipSet.Add(entry.Ip)
	}
	return ipSet
}

func (in *OpenGaussClusterSpec) RemoteIpSet() utils.Set {
	return utils.NewSetFromArray(in.RemoteIpList)
}

func (in *OpenGaussClusterSpec) IpArray() []string {
	ipArray := make([]string, 0)
	for _, entry := range in.IpList {
		ipArray = append(ipArray, entry.Ip)
	}
	return ipArray
}

func (in *OpenGaussClusterSpec) restoreChange(another *OpenGaussClusterSpec) bool {
	return in.RestoreFile != another.RestoreFile
}
func (in SyncState) Equals(another SyncState) bool {
	return in.IP == another.IP && in.Percent == another.Percent && in.State == another.State && in.Priority == another.Priority
}
func (in *OpenGaussCluster) ChangedConfig() map[string]string {
	return utils.GetChangedMap(in.Status.Spec.Config, in.GetValidSpec().Config)
}

func (in *OpenGaussCluster) GetConfigMapName() string {
	return fmt.Sprintf("og-%s-cm", in.Name)
}

func (in *OpenGaussCluster) GetPodName(ip string) string {
	return fmt.Sprintf("og-%s-pod-%s", in.Name, strings.Replace(ip, ".", "x", -1))
}

func (in *OpenGaussCluster) GetPVName(ip, pvType string) string {
	return fmt.Sprintf("%s-%s-pv", in.GetPodName(ip), pvType)
}

func (in *OpenGaussCluster) GetPVCName(ip, pvcType string) string {
	return fmt.Sprintf("%s-%s-pvc", in.GetPodName(ip), pvcType)
}
func (in *OpenGaussCluster) GetMostAvailableCount() int {
	spec := in.GetValidSpec()
	return utils.CalculateSyncCount(len(spec.IpList), len(spec.RemoteIpList))
}

func (in *OpenGaussCluster) GetServiceName(write bool) string {
	svcName := READ_SVC_NAME
	if write {
		svcName = WRITE_SVC_NAME
	}
	return fmt.Sprintf(svcName, in.Name)
}

func (in *OpenGaussCluster) GetSecretName() string {
	return fmt.Sprintf("%s-init-sc", in.Name)
}

func (in ScheduleConfig) Equals(another ScheduleConfig) bool {
	return in.ProcessTimeout == another.ProcessTimeout &&
		in.GracePeriod == another.GracePeriod &&
		in.Toleration == another.Toleration &&
		in.MostAvailableTimeout == another.MostAvailableTimeout &&
		in.PollingPeriod == another.PollingPeriod &&
		in.LivenessProbePeriod == another.LivenessProbePeriod &&
		in.ReadinessProbePeriod == another.ReadinessProbePeriod &&
		utils.CompareMaps(in.NodeLabels, another.NodeLabels)
}
func (in *OpenGaussClusterSpec) annotationChange(another *OpenGaussClusterSpec) bool {
	return !utils.CompareMaps(in.Annotations, another.Annotations)
}

func (in *OpenGaussClusterSpec) labelChange(another *OpenGaussClusterSpec) bool {
	return !utils.CompareMaps(in.Labels, another.Labels)
}

func (cluster *OpenGaussCluster) IsAnnotationChange() bool {
	if cluster.IsNew() {
		return false
	}
	lastSpec := cluster.Status.Spec.DeepCopy()
	currSpec := cluster.GetValidSpec()
	return currSpec.annotationChange(lastSpec)
}

func (cluster *OpenGaussCluster) IsLabelChange() bool {
	if cluster.IsNew() {
		return false
	}
	lastSpec := cluster.Status.Spec.DeepCopy()
	currSpec := cluster.GetValidSpec()
	return currSpec.labelChange(lastSpec)
}
func (cluster *OpenGaussCluster) IsBandwidthChange() bool {
	if cluster.IsNew() {
		return false
	}
	lastSpec := cluster.Status.Spec.DeepCopy()
	currSpec := cluster.GetValidSpec()
	return currSpec.BandWidth != lastSpec.BandWidth
}

func (in *OpenGaussCluster) GetPodIpByPodName(podName string) string {
	pos := strings.LastIndex(podName, "-pod-")
	return strings.Replace(podName[pos+5:], "x", ".", -1)
}
func (in *OpenGaussCluster) GetIpExtendIpMap() map[string]string {
	IpExtendIpMap := make(map[string]string)
	for _, entry := range in.Spec.IpList {
		IpExtendIpMap[entry.Ip] = entry.ExtendIp
	}
	return IpExtendIpMap
}
