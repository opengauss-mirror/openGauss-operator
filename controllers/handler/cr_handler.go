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

package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	//	"k8s.io/apimachinery/pkg/api/errors"

	opengaussv1 "opengauss-operator/api/v1"
	"opengauss-operator/controllers/service"
	"opengauss-operator/utils"
)

const (
	STATUS_UPDATE_RETRY_INTERVAL = 5
	STATUS_UPDATE_RETRY_LIMIT    = 10
)

/*
校验CR
方法参数：
	cluster：当前CR
返回值：
	CR是否合法
	是否可以继续监控和维护操作
处理逻辑：
	调用Validate()方法
	如果有错误
		如果是新集群，标记状态为invalid，在Status.Message中添加错误信息，返回“不合法”及“不继续操作”
		如果是已存在集群
			如果原状态不是invalid或者错误信息与此次不同，更新状态和错误信息，返回“不合法”及“可继续操作”
			返回“可继续操作”，表示operator可以根据当前CR之前的合法版本来维护现有资源
*/
func (s *syncHandlerImpl) Validate(cluster *opengaussv1.OpenGaussCluster) (bool, bool) {
	valid := true
	doReconcile := true
	err := cluster.Validate()
	if err != nil {
		s.Log.Info(err.Error())
		if cluster.IsNew() {
			s.updateStatus(cluster, opengaussv1.OpenGaussClusterStateInvalid, "", err.Error(), false)
			valid = false
			doReconcile = false
		} else {
			if cluster.Status.State != opengaussv1.OpenGaussClusterStateInvalid || cluster.Status.Message != err.Error() {
				s.updateStatus(cluster, opengaussv1.OpenGaussClusterStateInvalid, "", err.Error(), false)
			}
			valid = false
			doReconcile = true
		}
		s.eventService.ClusterValidateFailed(cluster, err)
	}
	return valid, doReconcile
}

/*
设置默认值
方法参数：
	cluster：当前CR
处理逻辑：
	调用DefaultSpec()填充默认值
	如果嗲用返回true，则表示有值修改，需要将修改更新到K8S
	如果当前CR状态是invalid，表示CR通过了校验但原状态不合法，修改状态为recorver，operator将检查集群资源
*/
func (s *syncHandlerImpl) SetDefault(cluster *opengaussv1.OpenGaussCluster) error {
	updateDefault := cluster.DefaultSpec()
	if updateDefault {
		if storedCluster, e := s.resourceService.GetCluster(cluster.Namespace, cluster.Name); e != nil {
			return e
		} else {
			storedCluster.Spec = *cluster.Spec.DeepCopy()
			if e := s.client.Update(context.TODO(), storedCluster); e != nil {
				s.Log.Error(e, fmt.Sprintf("[%s:%s]设置默认值失败", cluster.Namespace, cluster.Name))
				return e
			}
		}
		s.Log.Info(fmt.Sprintf("[%s:%s]设置默认值完成", cluster.Namespace, cluster.Name))
	}
	if !cluster.IsValid() {
		s.eventService.ClusterValidateFixed(cluster)
		s.updateStatus(cluster, opengaussv1.OpenGaussClusterStateRecover, "", "", false)
	}
	return nil
}

/*
检查集群是否需要处理
方法参数：
	cluster：当前集群
返回值：
	是否需要operator处理
方法逻辑：
	如果是新建CR，设置conditions为false，返回true
	如果CR设为维护状态
		如果前一个版本不是维护状态，返回true，否则返回false
	如果CR有修改
		如果资源修改，设置resourceCondition和instanceCondition为false
		如果数据库配置、RemoteIpList有修改，设置instanceCondition为false
		如果角色（主库，同城）有修改，设置instanceCondition和serviceCondition为false
		修改CR状态为update，返回true
	如果CR状态不是ready或invalid，返回true
	不是以上情况，则检查并返回所有数据库实例状态是否完备的结果
*/
func (s *syncHandlerImpl) isNeedReconcile(cluster *opengaussv1.OpenGaussCluster) bool {
	if cluster.IsNew() {
		s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, "")
		s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
		s.setCondition(cluster, opengaussv1.OpenGaussClusterServiceReady, corev1.ConditionFalse, "")
		s.updateStatus(cluster, opengaussv1.OpenGaussClusterStateCreate, "", "", false)
		s.eventService.ClusterCreate(cluster)
		return true
	}

	if cluster.MaintainModeChange() {
		return true
	} else if cluster.IsMaintaining() {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]集群处于维护模式", cluster.Namespace, cluster.Name))
		return false
	}

	if cluster.IsChanged() {
		state := opengaussv1.OpenGaussClusterStateUpdate
		if cluster.IsResourceChange() {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, "")
			s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
		}
		if cluster.IsDBConfigChange() || cluster.IsRemoteIpListChange() {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
		}
		if cluster.IsRoleChange() {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
			s.setCondition(cluster, opengaussv1.OpenGaussClusterServiceReady, corev1.ConditionFalse, "")
		}
		if cluster.IsRestoreChange() {
			s.setRestorePhase(cluster, opengaussv1.RestorePhasePrepare)
			state = opengaussv1.OpenGaussClusterStateRestore
		} else {
			s.eventService.ClusterUpdate(cluster)
		}
		s.updateStatus(cluster, state, "", "", false)
		return true
	}

	if cluster.Status.State != opengaussv1.OpenGaussClusterStateReady && cluster.Status.State != opengaussv1.OpenGaussClusterStateInvalid {
		s.Log.Info(fmt.Sprintf("[%s:%s]集群状态不是ready，而是%s", cluster.Namespace, cluster.Name, string(cluster.Status.State)))
		return true
	}
	if s.isClusterReconcileRequired(cluster) {
		s.eventService.ClusterRecover(cluster)
		return true
	} else {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]集群状态正常", cluster.Namespace, cluster.Name))
		if cluster.IsPrimary() {
			latestSyncStates, _ := s.getSyncState(cluster)
			if !compareSyncStates(latestSyncStates, cluster.Status.SyncStates) {
				s.updateStatus(cluster, cluster.Status.State, "", "", false)
			}
		}
		return false
	}
}

/*
检查集群实例状态
方法参数：
	cluster：当前CR
返回结果：
	集群是否需要处理
方法逻辑：
	遍历当前集群的所有Pod，存在以下情况则返回true
		存在状态为维护的实例
		存在状态不正常的实例
		现有集群的IP集合与Spec的IpList不吻合
		主集群无主或多主，或者主节点不在IpList中
		同城集群有主实例
		其他资源（ConfigMap、Secret、Service）丢失
	如果返回true，检查CR状态，对于invalid状态的CR，不更新状态，否则修改CR状态为recorver
*/
func (s *syncHandlerImpl) isClusterReconcileRequired(cluster *opengaussv1.OpenGaussCluster) bool {
	pods, err := s.resourceService.FindPodsByCluster(cluster, false)
	if err != nil {
		s.Log.Error(err, fmt.Sprintf("[%s:%s]查询集群Pod，发生错误", cluster.Namespace, cluster.Name))
		return false
	}
	ipSet := cluster.GetValidSpec().IpSet()
	podSet := utils.NewSet()
	primaryArray := make([]string, 0)
	notReadyCount := 0
	misMatchCount := 0
	maintenanceCount := 0
	for _, pod := range pods {
		podIP := pod.Status.PodIP
		podSet.Add(podIP)
		if !s.resourceService.IsPodReady(pod) {
			notReadyCount++
			continue
		}
		dbstate, err := s.dbService.CheckDBState(&pod)
		if err != nil {
			continue
		}
		if dbstate.IsPrimary() {
			primaryArray = append(primaryArray, podIP)
		}
		if dbstate.IsInMaintenance() {
			maintenanceCount++
		} else if dbstate.NeedConfigure() {
			notReadyCount++
		}
		if !validatePodLabel(pod, dbstate) {
			notReadyCount++
		}
		previousState := cluster.Status.PodState[podIP]
		if dbstate.PrintableString() != previousState {
			misMatchCount++
		}
	}
	needReconcile := false
	if maintenanceCount > 0 {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]有实例处于维护状态，需要更正", cluster.Namespace, cluster.Name))
		s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
		needReconcile = true
	}
	if !ipSet.Equals(podSet) {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]实例与规约不一致", cluster.Namespace, cluster.Name))
		s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, "")
		s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
		needReconcile = true
	} else {
		s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionTrue, "")
	}
	if cluster.IsPrimary() && (len(primaryArray) != 1 || !ipSet.Contains(primaryArray[0]) || cluster.Status.Primary != primaryArray[0]) {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]主节点状态与预期不符", cluster.Namespace, cluster.Name))
		s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
		s.setCondition(cluster, opengaussv1.OpenGaussClusterServiceReady, corev1.ConditionFalse, "")
		needReconcile = true
	} else if cluster.IsStandby() && len(primaryArray) != 0 {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]主节点状态与预期不符", cluster.Namespace, cluster.Name))
		s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
		s.setCondition(cluster, opengaussv1.OpenGaussClusterServiceReady, corev1.ConditionFalse, "")
		needReconcile = true
	} else {
		s.setCondition(cluster, opengaussv1.OpenGaussClusterServiceReady, corev1.ConditionTrue, "")
	}
	if notReadyCount > 0 {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]%d个实例状态异常", cluster.Namespace, cluster.Name, notReadyCount))
		s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
		needReconcile = true
	}
	if err := s.resourceService.CheckClusterArtifacts(cluster); err != nil {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]发生资源丢失，详情：%s", cluster.Namespace, cluster.Name, err.Error()))
		s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, "")
		s.setCondition(cluster, opengaussv1.OpenGaussClusterServiceReady, corev1.ConditionFalse, "")
		needReconcile = true
	}
	if cluster.IsValid() {
		if needReconcile {
			s.updateStatus(cluster, opengaussv1.OpenGaussClusterStateRecover, "", "", false)
		} else if misMatchCount > 0 {
			s.updateStatus(cluster, opengaussv1.OpenGaussClusterStateReady, "", "", false)
		}
	}
	return needReconcile
}

/*
检查Pod Label是否与数据库实例状态匹配
*/
func validatePodLabel(pod corev1.Pod, dbstate utils.DBState) bool {
	roleVal, exist := pod.Labels[service.OPENGAUSS_ROLE_KEY]
	if !exist {
		return false
	}
	if dbstate.IsPrimary() && roleVal != service.OG_DB_ROLE_PRIMARY {
		return false
	} else if dbstate.IsStandby() && roleVal != service.OG_DB_ROLE_STANDBY {
		return false
	}
	return true
}

func (s *syncHandlerImpl) setRestorePhase(cluster *opengaussv1.OpenGaussCluster, phase opengaussv1.RestorePhase) error {
	if cluster.Status.RestorePhase == phase {
		return nil
	}
	cluster.Status.RestorePhase = phase
	cluster.Status.LastUpdateTime = time.Now().Format(utils.TIME_FORMAT)
	if e := s.updateClusterStatus(cluster); e != nil {
		return e
	}
	if s.ensureStatusUpdate {
		return s.waitRestorePhaseUpdateComplete(cluster.Namespace, cluster.Name, phase, cluster.GetValidSpec().Schedule.ProcessTimeout)
	}
	return nil
}

/*
设置Condition
方法参数：
	cluster：当前CR
	conditionType：需要设置的condition
	status：condition的状态值
	message：condition的附加信息
*/
func (s *syncHandlerImpl) setCondition(cluster *opengaussv1.OpenGaussCluster, conditionType opengaussv1.OpenGaussClusterConditionType, status corev1.ConditionStatus, message string) error {
	if cluster.Status.Conditions == nil {
		cluster.Status.Conditions = make([]opengaussv1.OpenGaussClusterCondition, 0)
	}
	found := false
	change := false
	newCondition := opengaussv1.OpenGaussClusterCondition{
		Type:           conditionType,
		Status:         status,
		Message:        message,
		LastUpdateTime: time.Now().Format(utils.TIME_FORMAT),
	}
	conditions := cluster.Status.Conditions
	for index, condition := range conditions {
		if condition.Type == conditionType {
			if condition.Status != status {
				conditions[index] = newCondition
				change = true
			}
			found = true
			break
		}
	}
	if !found {
		conditions = append(conditions, newCondition)
		change = true
	}

	if change {
		cluster.Status.Conditions = conditions
		if e := s.updateClusterStatus(cluster); e != nil {
			return e
		}
		if s.ensureStatusUpdate {
			return s.waitConditionUpdateComplete(cluster.Namespace, cluster.Name, message, conditionType, status, cluster.GetValidSpec().Schedule.ProcessTimeout)
		}
	}
	return nil
}

/*
更新CR Status
方法参数：
	cluster：当前CR
	state：CR的期望状态
	primary：CR应记录的Primary IP
	message：CR Status的附加信息
	copySpec：如果是true，则将当前CR的Spec复制到Status中
*/
func (s *syncHandlerImpl) updateStatus(cluster *opengaussv1.OpenGaussCluster, state opengaussv1.OpenGaussClusterState, primary, message string, copySpec bool) error {
	update := false
	if cluster.Status.Message != message {
		cluster.Status.Message = message
		update = true
	}

	if cluster.IsStandby() && cluster.Status.Primary != "" {
		cluster.Status.Primary = ""
		update = true
	}

	if copySpec && cluster.IsChanged() {
		cluster.Status.Spec = *cluster.GetValidSpec()
		update = true
	}
	pods, err := s.resourceService.FindPodsByCluster(cluster, false)
	if err != nil {
		return err
	}
	ipSet := cluster.GetValidSpec().IpSet()
	currPodStates := make(map[string]string)
	notReadyPodState := make(map[string]string)
	for _, pod := range pods {
		podIP := pod.Status.PodIP
		podState := ""
		if s.resourceService.IsPodReady(pod) {
			dbstate, e := s.dbService.CheckDBState(&pod)
			if e != nil {
				podState = e.Error()
			} else {
				podState = dbstate.PrintableString()
			}
		} else {
			podState = fmt.Sprintf("Pod当前阶段是%s", string(pod.Status.Phase))
		}
		if podIP != "" {
			currPodStates[podIP] = podState
			ipSet.Remove(podIP)
		} else {
			notReadyPodState[pod.Name] = podState
		}
	}
	if !ipSet.IsEmpty() {
		for _, ip := range ipSet.ToArray() {
			podName := cluster.GetPodName(ip)
			if state, exist := notReadyPodState[podName]; exist {
				currPodStates[ip] = state
			} else {
				currPodStates[ip] = "unknown"
			}
		}
	}
	if !utils.CompareMaps(currPodStates, cluster.Status.PodState) {
		cluster.Status.PodState = currPodStates
		update = true
	}
	if state == opengaussv1.OpenGaussClusterStateReady && !cluster.IsValid() {
		state = opengaussv1.OpenGaussClusterStateInvalid
	}
	if cluster.Status.State != state {
		cluster.Status.State = state
		update = true
	}
	//查询当前各节点同步状态
	//如果查询出错，则保持之前的同步状态
	//currSyncStates用于后续校验更新CR status的结果
	currSyncStates := make([]opengaussv1.SyncState, 0)
	if latestSyncState, syncStateChanged, e := s.updateSyncStates(cluster); e != nil {
		return e
	} else {
		currSyncStates = latestSyncState
		if syncStateChanged {
			update = true
		}
	}
	if update {
		cluster.Status.LastUpdateTime = time.Now().Format(utils.TIME_FORMAT)
		if e := s.updateClusterStatus(cluster); e != nil {
			return e
		}
		if s.ensureStatusUpdate {
			return s.waitStatusUpdateComplete(cluster.Namespace, cluster.Name, message, state, copySpec, cluster.Status.Spec.DeepCopy(), cluster.Status.PodState, currSyncStates, cluster.GetValidSpec().Schedule.ProcessTimeout)
		} else if cluster.Status.State == opengaussv1.OpenGaussClusterStateReady {
			time.Sleep(time.Second * 30)
		}
	}
	return nil
}
func (s *syncHandlerImpl) updateSyncStates(cluster *opengaussv1.OpenGaussCluster) ([]opengaussv1.SyncState, bool, error) {
	currSyncStates := make([]opengaussv1.SyncState, 0)
	expectCount := cluster.GetMostAvailableCount()
	if expectCount == 0 {
		return currSyncStates, false, nil
	}
	update := false
	var err error
	if cluster.IsStandby() && len(cluster.Status.SyncStates) > 0 {
		cluster.Status.SyncStates = currSyncStates
		update = true
	} else if cluster.IsPrimary() {
		if cluster.IsNew() || cluster.IsChanged() || cluster.IsMaintaining() {
			return cluster.Status.SyncStates, false, nil
		}
		mostAvailableEnabled, e := s.getMostAvailableFlag(cluster)
		if e != nil {
			return currSyncStates, update, e
		}
		timeout := cluster.GetValidSpec().Schedule.MostAvailableTimeout
		var wait int32
		for {
			if querySyncStates, e := s.getSyncState(cluster); e != nil {
				s.Log.Error(e, fmt.Sprintf("[%s:%s]从主节点查询同步状态，发生错误", cluster.Namespace, cluster.Name))
				currSyncStates = cluster.Status.SyncStates
				break
			} else {
				currSyncStates = querySyncStates
				if !compareSyncStates(currSyncStates, cluster.Status.SyncStates) {
					cluster.Status.SyncStates = currSyncStates
					update = true
				}
				syncCount := getSyncInstanceCount(currSyncStates)
				if syncCount == expectCount && mostAvailableEnabled { //如果同步从节点数量达到预期，而most_available开启，则立刻关闭
					if e1 := s.processMostAvailableParameter(cluster, !mostAvailableEnabled); e1 != nil {
						err = e1
					} else {
						s.Log.Info(fmt.Sprintf("[%s:%s]同步从数量是%d，符合预期，关闭最大可用配置", cluster.Namespace, cluster.Name, syncCount))
					}
					break
				} else if syncCount < expectCount && !mostAvailableEnabled { //如果同步从节点数量未达到预期，most_available未开启，则重试直至超时，然后开启
					if wait >= timeout {
						if e1 := s.processMostAvailableParameter(cluster, !mostAvailableEnabled); e1 != nil {
							err = e1
						} else {
							s.Log.Info(fmt.Sprintf("[%s:%s]同步从数量是%d，未达到期望的%d，开启最大可用", cluster.Namespace, cluster.Name, syncCount, expectCount))
						}
						break
					} else {
						time.Sleep(time.Second * STATUS_UPDATE_RETRY_INTERVAL)
						wait += STATUS_UPDATE_RETRY_INTERVAL
					}
				} else {
					break
				}
			}
		}
	}
	return currSyncStates, update, err
}
func (s *syncHandlerImpl) processMostAvailableParameter(cluster *opengaussv1.OpenGaussCluster, enableMostAvailable bool) error {
	if primaryPod, err := s.resourceService.GetPod(cluster.Namespace, cluster.GetPodName(cluster.Status.Primary)); err != nil {
		return err
	} else {
		if changed, e := s.dbService.UpdateMostAvailable(primaryPod, enableMostAvailable); e != nil {
			return e
		} else if changed {
			// 修改most_available_sync参数后，无需重新重启db
			//	_, ok := s.dbService.RestartPrimary(primaryPod)
			//	if !ok {
			//		return fmt.Errorf("[%s:%s]未能将实例%s重启为主节点", cluster.Namespace, cluster.Name, primaryPod.Status.PodIP)
			//	}
			s.eventService.ConfigureMostAvailable(cluster, enableMostAvailable)
		}
	}
	return nil
}
func getSyncInstanceCount(syncStates []opengaussv1.SyncState) int {
	syncCount := 0
	for _, state := range syncStates {
		if state.State == OG_SYNC_STATE_SYNC {
			syncCount++
		}
	}
	return syncCount
}

func (s *syncHandlerImpl) getSyncState(cluster *opengaussv1.OpenGaussCluster) ([]opengaussv1.SyncState, error) {
	currStats := make([]opengaussv1.SyncState, 0)
	if cluster.Status.Primary == "" {
		return currStats, nil
	}
	primaryPod, err := s.resourceService.GetPod(cluster.Namespace, cluster.GetPodName(cluster.Status.Primary))
	if err != nil {
		return currStats, err
	}
	return s.dbService.QueryStandbyState(primaryPod)
}

func (s *syncHandlerImpl) getMostAvailableFlag(cluster *opengaussv1.OpenGaussCluster) (bool, error) {
	enabled := false
	if cluster.Status.Primary == "" {
		return enabled, nil
	}
	primaryPod, err := s.resourceService.GetPod(cluster.Namespace, cluster.GetPodName(cluster.Status.Primary))
	if err != nil {
		return enabled, err
	}
	return s.dbService.IsMostAvailableEnable(primaryPod)
}

func compareSyncStates(currStates, prevStats []opengaussv1.SyncState) bool {
	match := true
	if len(currStates) != len(prevStats) {
		match = false
	} else {
		for index, currState := range currStates {
			prevState := prevStats[index]
			if !currState.Equals(prevState) {
				match = false
				break
			}
		}
	}
	return match
}

/*
更新Condition后查询CR直至修改生效
方法参数：
	namespace： CR namspace
	name： CR name
	message： condition的附加信息
	conditionType：被修改的condition
	status：condition的期望状态值
方法逻辑：
	不断查询CR并与传入值对比，直至一致或超时
*/
func (s *syncHandlerImpl) waitConditionUpdateComplete(namespace, name, message string, conditionType opengaussv1.OpenGaussClusterConditionType, status corev1.ConditionStatus, timeout int32) error {
	retryCount := int32(0)
	for {
		cluster, err := s.resourceService.GetCluster(namespace, name)
		if err == nil {
			conditions := cluster.Status.Conditions
			for _, condition := range conditions {
				if condition.Type == conditionType && condition.Status == status && condition.Message == message {
					s.Log.Info(fmt.Sprintf("[%s:%s]集群条件更新成功，条件[%s]=%s", namespace, name, string(conditionType), string(status)))
					return nil
				}
			}
		}
		if retryCount*STATUS_UPDATE_RETRY_INTERVAL >= timeout {
			return fmt.Errorf("[%s:%s]集群条件%s更新错误，重试次数达到上限%d次，终止重试", namespace, name, string(conditionType), timeout)
		} else {
			retryCount++
			s.Log.V(1).Info(fmt.Sprintf("[%s:%s]集群条件%s更新未完成，将于%d秒后尝试第%d次重试", namespace, name, string(conditionType), STATUS_UPDATE_RETRY_INTERVAL, retryCount))
			time.Sleep(time.Second * STATUS_UPDATE_RETRY_INTERVAL)
		}
	}
}

/*
更新Status后查询CR直至修改生效
方法参数：
	namespace： CR namspace
	name： CR name
	message： CR Status的附加信息
	state：CR Status的期望状态值
	copySpec：是否复制Spec到Status
	podState：当前所有Pod中实例的状态
方法逻辑：
	不断查询CR并与传入值对比，直至一致或超时
*/
func (s *syncHandlerImpl) waitStatusUpdateComplete(namespace, name, message string, state opengaussv1.OpenGaussClusterState, copySpec bool, expectSpec *opengaussv1.OpenGaussClusterSpec, podState map[string]string, syncStates []opengaussv1.SyncState, timeout int32) error {
	retryCount := int32(0)
	for {
		cluster, err := s.resourceService.GetCluster(namespace, name)
		if err == nil {
			complete := cluster.Status.State == state && cluster.Status.Message == message && comparePodStates(podState, cluster.Status.PodState) && compareSyncStates(syncStates, cluster.Status.SyncStates)
			if copySpec {
				complete = complete && expectSpec.Equals(cluster.Status.Spec.DeepCopy())
			}
			if complete {
				s.Log.Info(fmt.Sprintf("[%s:%s]集群状态更新成功，状态为%s", namespace, name, string(state)))
				return nil
			}
		}
		if retryCount*STATUS_UPDATE_RETRY_INTERVAL >= timeout {
			return fmt.Errorf("[%s:%s]集群状态更新错误，重试次数达到上限%d次，终止重试", namespace, name, timeout)
		} else {
			retryCount++
			s.Log.V(1).Info(fmt.Sprintf("[%s:%s]集群状态更新未完成，将于%d秒后尝试第%d次重试", namespace, name, STATUS_UPDATE_RETRY_INTERVAL, retryCount))
			time.Sleep(time.Second * STATUS_UPDATE_RETRY_INTERVAL)
		}
	}
}

/*
等待数据恢复阶段更新完成
方法参数：
	namespace：Redis命名空间
	name：Redis名称
	phase：数据恢复阶段
*/
func (s *syncHandlerImpl) waitRestorePhaseUpdateComplete(namespace, name string, phase opengaussv1.RestorePhase, timeout int32) error {
	retryCount := int32(0)
	for {
		cluster, err := s.resourceService.GetCluster(namespace, name)
		if err == nil && cluster.Status.RestorePhase == phase {
			s.Log.Info(fmt.Sprintf("[%s:%s]数据恢复阶段更新成功，阶段为%s", namespace, name, string(phase)))
			return nil
		}
		if retryCount*STATUS_UPDATE_RETRY_INTERVAL >= timeout {
			return fmt.Errorf("[%s:%s]数据恢复阶段更新错误，重试次数达到上限%d次，终止重试", namespace, name, timeout)
		} else {
			retryCount++
			s.Log.V(1).Info(fmt.Sprintf("[%s:%s]数据恢复阶段更新未完成，将于%d秒后尝试第%d次重试", namespace, name, STATUS_UPDATE_RETRY_INTERVAL, retryCount))
			time.Sleep(time.Second * STATUS_UPDATE_RETRY_INTERVAL)
		}
	}
}
func (s *syncHandlerImpl) updateClusterStatus(cluster *opengaussv1.OpenGaussCluster) error {
	if storedCluster, e := s.resourceService.GetCluster(cluster.Namespace, cluster.Name); e != nil {
		return e
	} else {
		storedCluster.Status = *cluster.Status.DeepCopy()
		if e := s.client.Status().Update(context.TODO(), storedCluster); e != nil {
			return e
		}
	}
	return nil
}
func comparePodStates(expectStates, actualStates map[string]string) bool {
	for ip, state := range expectStates {
		actualState, exist := actualStates[ip]
		if !exist || state != actualState {
			return false
		}
	}
	return true
}

func ipArrayToString(array []string) string {
	var b strings.Builder
	for _, val := range array {
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		b.WriteString(val)
	}
	return "[" + b.String() + "]"
}
