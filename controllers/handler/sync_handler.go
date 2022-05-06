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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opengaussv1 "opengauss-operator/api/v1"
	"opengauss-operator/controllers/service"
	"opengauss-operator/utils"
)

const (
	OG_SYNC_STATE_SYNC      = "Sync"
	OG_SYNC_STATE_POTENTIAL = "Potential"
)

type SyncHandler interface {
	Validate(cluster *opengaussv1.OpenGaussCluster) (bool, bool)
	SetDefault(cluster *opengaussv1.OpenGaussCluster) error
	SyncCluster(cluster *opengaussv1.OpenGaussCluster) error
}

type syncHandlerImpl struct {
	client             client.Client
	Log                logr.Logger
	resourceService    service.IResourceService
	dbService          service.IDBService
	eventService       service.EventService
	ensureStatusUpdate bool
}

func NewSyncHandler(client client.Client, logger logr.Logger, eventRecorder record.EventRecorder, ensureStatusUpdate bool) SyncHandler {
	return &syncHandlerImpl{
		client:             client,
		Log:                logger,
		resourceService:    service.NewResourceService(client, logger),
		dbService:          service.NewDBService(client, logger),
		eventService:       service.NewEventService(eventRecorder),
		ensureStatusUpdate: ensureStatusUpdate,
	}
}

/*
根据OpenGaussClusterSpec维护集群，包括
	1 维护集群资源，包括ConfigMap，Secret，Service，Pod和相应的PVC/PV
	2 维护Pod中的OpenGauss实例的主从结构
方法参数：
	cluster：当前CR
*/
func (s *syncHandlerImpl) SyncCluster(cluster *opengaussv1.OpenGaussCluster) error {
	s.Log.Info(fmt.Sprintf("[%s:%s]开始处理集群", cluster.Namespace, cluster.Name))
	if s.isNeedReconcile(cluster) {
		if err := s.resourceService.EnsureConfigMaps(cluster); err != nil {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, err.Error())
			s.updateStatus(cluster, cluster.Status.State, "", err.Error(), false)
			return err
		}
		if err := s.resourceService.EnsureSecret(cluster); err != nil {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, err.Error())
			s.updateStatus(cluster, cluster.Status.State, "", err.Error(), false)
			return err
		}
		if err := s.resourceService.EnsureServices(cluster); err != nil {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, err.Error())
			s.updateStatus(cluster, cluster.Status.State, "", err.Error(), false)
			return err
		}
		if err := s.ensureDBCluster(cluster); err != nil {
			s.updateStatus(cluster, cluster.Status.State, "", err.Error(), false)
			return err
		}
	}
	s.Log.Info(fmt.Sprintf("[%s:%s]集群处理完成", cluster.Namespace, cluster.Name))
	return nil
}

/*
根据OpenGaussClusterSpec维护OpenGauss数据库集群
方法参数：
	cluster：当前CR
方法逻辑：
	1 如果集群标记为维护状态，不做操作
	2 根据Spec.IpList，确保每个IpNodeEntry都有一个对应的Pod，确保存在一个Primary，其他实例为Standby，所有实例组成一个完整集群
		完成后，可能存在前一个版本设置的，但当前版本CR中被删除的IP所对应的Pod
	3 清理与当前Spec的IpList不匹配的Pod
	4 如果当前Spec相对于前一个版本有资源升级，则对Pod进行滚动升级
	5 如果当前Spec相对于前一个版本有数据库配置变更，则修改所有实例的配置参数
完成后，标记CR状态为ready
支持对状态为invalid的CR进行资源维护，通过OpenGaussCluster.GetValidSpec()方法，获取CR修改前的Spec，用于资源和集群实力的维护
*/
func (s *syncHandlerImpl) ensureDBCluster(cluster *opengaussv1.OpenGaussCluster) error {
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]开始维护数据库集群", cluster.Namespace, cluster.Name))

	if cluster.IsMaintainStart() {
		s.Log.Info(fmt.Sprintf("[%s:%s]集群维护模式开启", cluster.Namespace, cluster.Name))
		s.ensureMaintenance(cluster)
		s.eventService.ClusterMaintainStart(cluster)
		return nil
	} else if cluster.IsMaintainEnd() {
		s.Log.Info(fmt.Sprintf("[%s:%s]集群维护模式结束", cluster.Namespace, cluster.Name))
		s.eventService.ClusterMaintainComplete(cluster)
	}
	if e := s.ensureSpecCluster(cluster); e != nil {
		s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, e.Error())
		s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, e.Error())
		return e
	} else {
		s.setCondition(cluster, opengaussv1.OpenGaussClusterServiceReady, corev1.ConditionTrue, "")
	}
	if e := s.cleanupCluster(cluster); e != nil {
		s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, e.Error())
		return e
	} else {
		s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionTrue, "")
		s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionTrue, "")
	}
	if s.upgradeRequired(cluster) {
		s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, "")
		if e := s.upgradeCluster(cluster); e != nil {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionFalse, e.Error())
			return e
		} else {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterResourceReady, corev1.ConditionTrue, "")
		}
	} else if cluster.IsDBConfigChange() {
		s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, "")
		if e := s.updateDBConfig(cluster); e != nil {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionFalse, e.Error())
			return e
		} else {
			s.setCondition(cluster, opengaussv1.OpenGaussClusterInstancesReady, corev1.ConditionTrue, "")
		}
	}
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]数据库集群处理完成", cluster.Namespace, cluster.Name))
	if cluster.IsValid() {
		s.updateStatus(cluster, opengaussv1.OpenGaussClusterStateReady, "", "", true)
		s.eventService.ClusterReady(cluster)
	}
	return nil
}

/*
当CR.Spec.Maintenance设置为true时，Operator在所有Pod上添加标记文件，允许手动停止数据库进程而不会造成Pod重启
方法参数：
	cluster：当前CR
*/
func (s *syncHandlerImpl) ensureMaintenance(cluster *opengaussv1.OpenGaussCluster) {
	pods, _ := s.resourceService.FindPodsByCluster(cluster, false)
	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		s.dbService.AddMaintenanceFlag(&pod)
	}
	s.updateStatus(cluster, opengaussv1.OpenGaussClusterStateMaintain, "", "", true)
}

/*
确保有唯一的Primary实例
方法参数：
	cluster：当前CR
	pods： 当前集群处于running状态的所有Pod
	ipArray：ip数组，包括CR.Spec.IpList的所有IP和已存在的Pod的IP
	newPvcPods：新建PVC的IP数组
*/
func (s *syncHandlerImpl) ensurePrimary(cluster *opengaussv1.OpenGaussCluster, pods []corev1.Pod, ipArray []string, newPvcPods utils.Set) error {
	//如果没有Pod则无需设置Primary实例
	if len(pods) == 0 {
		return nil
	}
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]开始维护数据库主节点", cluster.Namespace, cluster.Name))
	//筛选出primary实例
	primaryPods := make([]corev1.Pod, 0)
	//ipSet初始化为IpList的集合，并在循环中添加为集群当前所有Pod的IP
	for _, pod := range pods {
		if dbstate, err := s.dbService.CheckDBState(&pod); err != nil {
			continue
		} else if dbstate.IsPrimary() {
			primaryPods = append(primaryPods, pod)
		}
	}

	//根据Primary的数目进行处理
	//多主：主集群有超过一个Primary，或同城集群有一个Primary
	//一主：主集群有一个Primary
	//无主：主集群没有Primary
	if len(primaryPods) > 1 || (len(primaryPods) == 1 && cluster.IsStandby()) {
		//处理多主
		if err := s.processMultiplePrimary(cluster, primaryPods, ipArray); err != nil {
			return err
		}
	} else if len(primaryPods) == 1 {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]主节点位于Pod %s", cluster.Namespace, cluster.Name, primaryPods[0].Name))
		primaryPod := primaryPods[0]
		cluster.Status.Primary = primaryPod.Status.PodIP
		if cluster.IsIpListChange() || cluster.IsRemoteIpListChange() {
			_, ok, err := s.configDBInstance(cluster, &primaryPod, ipArray, true, true, false)
			if !ok {
				s.Log.Error(err, fmt.Sprintf("[%s:%s]配置位于Pod %s的数据库实例，发生错误", cluster.Namespace, cluster.Name, primaryPod.Name))
				return err
			}
		}
	} else if cluster.IsPrimary() {
		//处理无主
		err := s.processNoPrimary(cluster, pods, ipArray, newPvcPods)
		if err != nil {
			return err
		}
	}
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]数据库主节点维护完成", cluster.Namespace, cluster.Name))
	return nil
}

/*
根据stateMap中记录的实例角色，更新Pod Label，清理维护标记文件
方法参数：
	cluster：当前CR
*/
func (s *syncHandlerImpl) updatePodLabels(cluster *opengaussv1.OpenGaussCluster) error {
	// update read/write label for each pod
	pods, _ := s.resourceService.FindPodsByCluster(cluster, false)
	for _, pod := range pods {
		if pod.Status.PodIP == "" {
			continue
		}
		if !s.resourceService.IsPodReady(pod) {
			continue
		}
		dbstate, err := s.dbService.CheckDBState(&pod)
		if err != nil {
			s.Log.Error(err, fmt.Sprintf("[%s:%s]查询位于Pod %s的数据库状态，发生错误", cluster.Namespace, cluster.Name, pod.Name))
			continue
		}
		if err := s.addRoleLabelToPod(cluster, pod.Status.PodIP, dbstate.IsPrimary()); err != nil {
			return err
		}
		if dbstate.IsInMaintenance() {
			if _, ok := s.dbService.RemoveMaintenanceFlag(&pod); !ok {
				return fmt.Errorf("[%s:%s]未能移除位于Pod %s的维护标记", cluster.Namespace, cluster.Name, pod.Name)
			}
		}
	}
	return nil
}

/*
处理多主问题
方法参数：
	cluster：当前CR
	primaryPods：实例角色为Primary的Pod数组
	ipArray：ip数组，包括CR.Spec.IpList的所有IP和已存在的Pod的IP
对于主集群，确保只有一个Primary
方法逻辑：
	如果存在与CR.Status.Primary匹配的Pod，则以此为Primary，将其他主实例重启为Pending，由后续逻辑处理
	如果不存在与CR.Status.Primary匹配的Pod，查询所有Primary的LSN,选择最大的一个作为Primary，其余重启为Pending
	如果CR有IpList或RemoteIpList的改变，唯一的Primary需要重新配置
对于同城集群，确保没有Primary
方法逻辑：
	将所有Primary重启为Standby
*/
func (s *syncHandlerImpl) processMultiplePrimary(cluster *opengaussv1.OpenGaussCluster, primaryPods []corev1.Pod, ipArray []string) error {
	s.Log.Info(fmt.Sprintf("[%s:%s]检测到多个数据库主节点", cluster.Namespace, cluster.Name))
	if cluster.IsPrimary() {
		matchWithStatus := false
		//查看是否有Pod的IP与CR.Status.Primary相同
		for _, primaryPod := range primaryPods {
			if primaryPod.Status.PodIP == cluster.Status.Primary {
				matchWithStatus = true
				break
			}
		}
		//没有IP与CR.Status.Primary相同的Pod，从现有的Priamry中选择LSN最大的一个
		if !matchWithStatus {
			maxLsnPod := s.dbService.FindPodWithLargestLSN(primaryPods, "")
			cluster.Status.Primary = maxLsnPod.Status.PodIP
		}
	} else {
		//对于同城集群，Status.Primary应为空
		cluster.Status.Primary = ""
	}
	//清理多主
	for _, primaryPod := range primaryPods {
		if primaryPod.Status.PodIP != cluster.Status.Primary {
			//删除Pod Label
			s.removeRoleLabelFromPod(cluster, primaryPod.Status.PodIP)
			//重启实例
			if cluster.IsPrimary() {
				// 主集群，除选定的主节点外其余主节点重启为pending状态
				// 确保原有远程连接断开，实例稍后通过basebackup重新建立主从关系
				s.dbService.StopDB(&primaryPod)
				s.dbService.StartPending(&primaryPod)
			} else {
				// 同城集群，通常场景为主备集群切换
				// 将原主重启为standby，可能发生因无主而进入“need repair”状态
				// 待原备集群切换为主集群，选出新主节点后，所有节点即可恢复正常
				s.dbService.RestartStandby(&primaryPod)
			}
		} else if cluster.IsIpListChange() || cluster.IsRemoteIpListChange() {
			_, ok, err := s.configDBInstance(cluster, &primaryPod, ipArray, true, true, true)
			if !ok {
				return err
			}
		}
	}
	if cluster.IsPrimary() {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]保持位于Pod %s的数据库实例为主节点", cluster.Namespace, cluster.Name, cluster.Status.Primary))
	} else {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]保持无主状态", cluster.Namespace, cluster.Name))
	}
	s.Log.Info(fmt.Sprintf("[%s:%s]处理多主故障完成", cluster.Namespace, cluster.Name))
	return nil
}

/*
处理无主问题
方法参数：
	cluster：当前CR
	pods：当前集群所有Pod的数组
	ipArray：ip数组，包括CR.Spec.IpList的所有IP和已存在的Pod的IP
	newPvcPods：新建PVC的IP数组
方法逻辑：
	将Pod数组根据DB状态分组，Standby实例一组，其他一组
	将其他组所有不在newPVCPods中的实例启动为Standby，并加入Standby组
	如果Standby组不为空则选出LSN最大者，否则指定为pods的第一个元素
	将选定的实例配置为Primary
*/
func (s *syncHandlerImpl) processNoPrimary(cluster *opengaussv1.OpenGaussCluster, pods []corev1.Pod, ipArray []string, newPVCPods utils.Set) error {
	s.Log.Info(fmt.Sprintf("[%s:%s]当前数据库实例中没有主节点", cluster.Namespace, cluster.Name))

	standbyPods := make([]corev1.Pod, 0)
	otherPods := make([]corev1.Pod, 0)
	stateMap := make(map[string]utils.DBState)
	for _, pod := range pods {
		if dbstate, err := s.dbService.CheckDBState(&pod); err != nil {
			continue
		} else {
			stateMap[pod.Status.PodIP] = dbstate
			if dbstate.IsStandby() {
				standbyPods = append(standbyPods, pod)
			} else {
				otherPods = append(otherPods, pod)
			}
		}
	}

	if len(otherPods) > 0 {
		for _, pod := range otherPods {
			if !newPVCPods.Contains(pod.Status.PodIP) {
				if _, started := s.dbService.StartDBToStandby(&pod); started {
					standbyPods = append(standbyPods, pod)
				}
			}
		}
	}
	selectedPod := corev1.Pod{}
	if len(standbyPods) > 0 {
		selectedPod = s.dbService.FindPodWithLargestLSN(standbyPods, cluster.Status.Primary)
	} else {
		selectedPod = pods[0]
	}
	if s.checkStandbySyncState(cluster, selectedPod.Status.PodIP) {
		s.Log.Info(fmt.Sprintf("[%s:%s]位于Pod %s的实例被选为主节点", cluster.Namespace, cluster.Name, selectedPod.Name))
		// start it to primary
		_, ok, _ := s.configDBInstance(cluster, &selectedPod, ipArray, true, true, true)
		if ok {
			s.Log.Info(fmt.Sprintf("[%s:%s]位于Pod %s的实例被配置为主节点", cluster.Namespace, cluster.Name, selectedPod.Name))
			cluster.Status.Primary = selectedPod.Status.PodIP
			return nil
		} else {
			return fmt.Errorf("[%s:%s]未能配置位于Pod %s的实例为主节点", cluster.Namespace, cluster.Name, selectedPod.Name)
		}
	} else {
		err := fmt.Errorf("[%s:%s]选择的实例%s数据不完整，不能配置为主节点", cluster.Namespace, cluster.Name, selectedPod.Status.PodIP)
		s.eventService.ClusterFailed(cluster, err)
		s.updateStatus(cluster, opengaussv1.OpenGaussClusterStateFailed, "", err.Error(), false)
		return err
	}
}

/*
根据CR.Spec配置集群
方法参数：
	cluster：当前CR
方法逻辑：
	确保CR.Spec.IpList中的每个元素（IpNodeEntry）都有一个对应的Pod
	根据Pod的实际情况（是否新建，是否新建PVC）进行分组
	等待所有Pod达到running状态
	在running状态的Pod中选出一个实例Primary
	将其他实例配置为Standby
	更新所有Pod的Label
*/
func (s *syncHandlerImpl) ensureSpecCluster(cluster *opengaussv1.OpenGaussCluster) error {
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]开始根据规约维护集群", cluster.Namespace, cluster.Name))
	// ensure PVC and Pod for each IpNodeEntry
	// if there is storage change
	// PVC will be modified
	newPvcPods := utils.NewSet()
	podCreateErrors := make([]error, 0)
	for _, entry := range cluster.GetValidSpec().IpList {
		pvcCreated, _, err := s.resourceService.EnsurePodResource(cluster, entry)
		if err != nil {
			s.Log.Error(err, fmt.Sprintf("[%s:%s]在工作节点%s上创建名为%s的Pod，发生错误", cluster.Namespace, cluster.Name, entry.NodeName, cluster.GetPodName(entry.Ip)))
			podCreateErrors = append(podCreateErrors, err)
			continue
		}
		if pvcCreated {
			newPvcPods.Add(entry.Ip)
		}
	}
	//如果错误数量等于IpList长度，即所有Pod错误，将错误向上抛出
	if len(podCreateErrors) == len(cluster.GetValidSpec().IpList) {
		errorMessage := make([]string, 0)
		for _, e := range podCreateErrors {
			errorMessage = append(errorMessage, e.Error())
		}
		return fmt.Errorf("[%s:%s]创建Pod发生错误，详情：%s", cluster.Namespace, cluster.Name, utils.StringArrayToString(errorMessage))
	}

	pods, ok := s.resourceService.WaitPodsRunning(cluster)
	//如果等待所有Pod状态变为running超时
	//	没有Pod可用，则抛出错误
	//	否则以可用pod组成集群
	if !ok {
		if len(pods) == 0 {
			return fmt.Errorf("[%s:%s]没有可用的Pod，进程终止", cluster.Namespace, cluster.Name)
		} else {
			s.Log.Info(fmt.Sprintf("[%s:%s]节点数目未达到预期，将以%d个节点组建集群", cluster.Namespace, cluster.Name, len(pods)))
		}
	}

	ipSet := cluster.GetValidSpec().IpSet()
	for _, pod := range pods {
		ipSet.Add(pod.Status.PodIP)
	}
	ipArr := ipSet.ToArray()
	//主集群选主
	if e := s.ensurePrimary(cluster, pods, ipArr, newPvcPods); e != nil {
		return e
	}

	if cluster.RestoreRequired() {
		if e := s.restoreDataFromFile(cluster, ipArr, newPvcPods); e != nil {
			return e
		}
		pods, _ = s.resourceService.WaitPodsRunning(cluster)
	}

	//配置所有Standby
	if err := s.ensureStandbyInstances(cluster, pods, ipArr, newPvcPods); err != nil {
		return err
	}
	//更新PodLabel
	if err := s.updatePodLabels(cluster); err != nil {
		return err
	}
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]根据规约维护集群结束", cluster.Namespace, cluster.Name))
	return nil
}

func (s *syncHandlerImpl) restoreDataFromFile(cluster *opengaussv1.OpenGaussCluster, ipArray []string, newPvcPods utils.Set) error {
	s.Log.Info(fmt.Sprintf("[%s:%s]开始数据恢复", cluster.Namespace, cluster.Name))
	s.setRestorePhase(cluster, opengaussv1.RestorePhaseRunning)
	primary := cluster.Status.Primary
	primaryPod, err := s.resourceService.GetPod(cluster.Namespace, cluster.GetPodName(primary))
	if err != nil {
		return err
	}
	state, success := s.dbService.RestoreDB(primaryPod, cluster.GetValidSpec().RestoreFile)
	if success {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]在Pod %s上完成数据恢复", cluster.Namespace, cluster.Name, primaryPod.Name))
		_, success = s.dbService.StartPending(primaryPod)
		if success {
			_, configured, _ := s.configDBInstance(cluster, primaryPod, ipArray, true, true, false)
			if !configured {
				return fmt.Errorf("[%s:%s]在Pod %s上配置数据库实例失败", cluster.Namespace, cluster.Name, primaryPod.Name)
			}
		} else {
			return fmt.Errorf("[%s:%s]在Pod %s上启动数据库实例失败", cluster.Namespace, cluster.Name, primaryPod.Name)
		}
	} else {
		if state.IsRestoreFailed() {
			s.eventService.InstanceRestoreFail(cluster, primary)
			s.Log.V(3).Info(fmt.Sprintf("[%s:%s]在Pod %s上恢复数据失败，清理数据，稍后重试", cluster.Namespace, cluster.Name, primaryPod.Name))
			if e := s.resourceService.CleanPodResource(cluster, primary); e != nil {
				return e
			}
		}
		return fmt.Errorf("[%s:%s]在Pod %s上复制数据失败", cluster.Namespace, cluster.Name, primaryPod.Name)
	}
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]位于Pod %s上的实例完成配置", cluster.Namespace, cluster.Name, primaryPod.Name))
	pods, err := s.resourceService.FindPodsByCluster(cluster, false)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		podIP := pod.Status.PodIP
		if podIP == primary || podIP == "" || newPvcPods.Contains(podIP) {
			continue
		}
		if e := s.resourceService.CleanPodResource(cluster, podIP); e != nil {
			return e
		}
	}
	for _, entry := range cluster.GetValidSpec().IpList {
		if pvcCreated, _, e := s.resourceService.EnsurePodResource(cluster, entry); e != nil {
			s.Log.Error(e, fmt.Sprintf("[%s:%s]在工作节点%s上创建名为%s的Pod，发生错误", cluster.Namespace, cluster.Name, entry.NodeName, cluster.GetPodName(entry.Ip)))
			continue
		} else if pvcCreated {
			newPvcPods.Add(entry.Ip)
		}
	}
	s.setRestorePhase(cluster, opengaussv1.RestorePhaseSucceeded)
	s.eventService.ClusterRestore(cluster)
	return nil
}

//过滤出当前所有具备完整数据的节点
//方法仅适用于primary存在的情况，输入和返回结果中包含primary
func (s *syncHandlerImpl) getSyncPods(cluster *opengaussv1.OpenGaussCluster, pods []corev1.Pod) []corev1.Pod {
	syncPods := make([]corev1.Pod, 0)
	syncStates, err := s.getSyncState(cluster)
	if err != nil {
		syncStates = cluster.Status.SyncStates
	}
	syncSet := utils.NewSet()
	for _, state := range syncStates {
		if state.State == OG_SYNC_STATE_SYNC {
			syncSet.Add(state.IP)
		}

	}
	for _, pod := range pods {
		podIP := pod.Status.PodIP
		if podIP == cluster.Status.Primary || syncSet.Contains(podIP) {
			syncPods = append(syncPods, pod)
		}
	}
	return syncPods
}

//检查给定IP的实例是否是同步从
//使用status.SyncStates做检查
//如果实例是被记录的primary或同步standby则返回true
func (s *syncHandlerImpl) checkStandbySyncState(cluster *opengaussv1.OpenGaussCluster, podIP string) bool {
	if cluster.Status.Primary == podIP || cluster.Status.Primary == "" {
		return true
	}
	syncStates := cluster.Status.SyncStates
	synced := false
	if len(syncStates) > 0 {
		for _, state := range syncStates {
			if state.IP == podIP && state.State == OG_SYNC_STATE_SYNC {
				synced = true
				break
			}
		}
	}
	return synced
}

/*
自动修复状态不是Normal的Standby实例
方法参数：
	cluster：当前CR
	pod：故障Pod
返回值：
	当前pod（pod可能重启，需重新查询获取）
	当前Pod的实例状态
方法逻辑：
	如果当前Pod角色为Standby，状态不是Normal，也不是Disconnected，则需处理
	通过停止数据库进程，并清除维护标记（停止进程会自动添加维护标记），使得Pod重启
	观察实例状态是否恢复，重试间隔10秒，超时限制5分钟
*/
func (s *syncHandlerImpl) fixStandbyInstance(cluster *opengaussv1.OpenGaussCluster, pod corev1.Pod) (corev1.Pod, utils.DBState) {
	podIP := pod.Status.PodIP
	dbstate, err := s.dbService.CheckDBState(&pod)
	if err != nil {
		return pod, dbstate
	}
	if !dbstate.IsNormal() && !dbstate.IsDisconnected() {
		s.dbService.StopDB(&pod)
		s.dbService.RemoveMaintenanceFlag(&pod)
	}
	retryCount := 0
	for {
		fixedPod, err := s.resourceService.GetPod(cluster.Namespace, cluster.GetPodName(podIP))
		if err == nil {
			dbstate, _ = s.dbService.CheckDBState(fixedPod)
			if dbstate.IsPending() {
				dbstate, _ = s.dbService.StartStandby(fixedPod)
			}
			if dbstate.IsStandby() && dbstate.IsNormal() {
				if dbstate.IsInMaintenance() {
					dbstate, _ = s.dbService.RemoveMaintenanceFlag(fixedPod)
				}
				s.eventService.FixStandbyComplete(cluster, podIP, dbstate.PrintableString())
				return *fixedPod, dbstate
			}
		}
		if retryCount > utils.RETRY_LIMIT {
			s.Log.Info(fmt.Sprintf("[%s:%s]修复Pod状态重试达到上限%d秒，停止重试，当前状态是%s", cluster.Namespace, cluster.Name, utils.RETRY_LIMIT*utils.RETRY_INTERVAL, dbstate.PrintableString()))
			s.eventService.FixStandbyFail(cluster, podIP, dbstate.PrintableString())
			return *fixedPod, dbstate
		} else {
			retryCount++
			s.Log.V(1).Info(fmt.Sprintf("[%s:%s]修复Pod状态未完成，将于%d秒后进行第%d次重试", cluster.Namespace, cluster.Name, utils.RETRY_INTERVAL, retryCount))
		}
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
}

/*
配置集群（除Primary以外）的所有实例
方法参数：
	cluster：当前CR
	pods：当前所有Pod的数组
	newPvcPods：新建PVC的Pod的IP集合
	newPods：新建的Pod的IP集合
方法逻辑：
	在之前的configPrimary()中已经选出并配置了Primary
	遍历所有Pod，对于角色为Standby的实例，根据IpList和RemoteIpList更新连接信息
	将Pod分为三组：状态正常的Standby实例，状态不正常的Standby实例，和Pending实例
	状态正常的Standby实例作为复制数据的源节点备选，其他实例需要通过basebackup同步数据
	遍历需要复制数据的实例
		从当前的源节点中通过比较LSN选出一个作为源节点，当前实例作为目标节点，进行basebackup
		完成后当前目标节点从需要复制的数组中移除，加入源节点数组
*/
func (s *syncHandlerImpl) ensureStandbyInstances(cluster *opengaussv1.OpenGaussCluster, pods []corev1.Pod, ipArray []string, newPvcPods utils.Set) error {
	primaryIP := cluster.Status.Primary
	if (cluster.IsPrimary() && len(pods) > 1) || cluster.IsStandby() {
		podsToBuild := make([]corev1.Pod, 0)
		podsBuilt := make([]corev1.Pod, 0)
		for _, pod := range pods {
			podIP := pod.Status.PodIP
			if podIP == primaryIP {
				continue
			}
			dbstate, err := s.dbService.CheckDBState(&pod)
			if err != nil {
				continue
			}
			if !dbstate.IsProcessExist() {
				if state, started := s.dbService.StartPending(&pod); started {
					dbstate = state
				} else {
					podsToBuild = append(podsToBuild, pod)
				}
			}
			//主库转为同城集群，所有实例重启为pending状态，等待basebackup同步数据
			if cluster.IsStandby() && cluster.IsRoleChange() && cluster.IsRemoteIpListChange() {
				if !dbstate.IsPending() {
					dbstate, _ = s.dbService.RestartPending(&pod)
				}
			} else if !newPvcPods.Contains(podIP) && (cluster.IsRemoteIpListChange() || cluster.IsIpListChange()) {
				//有IP变动，需要配置连接信息
				dbstate, _, _ = s.configDBInstance(cluster, &pod, ipArray, false, false, false)
			}

			//通过重启尝试修复故障
			if cluster.IsPrimary() && dbstate.IsStandby() && !dbstate.IsNormal() && dbstate.IsBackupComplete() && !newPvcPods.Contains(podIP) {
				newPod, newstate := s.fixStandbyInstance(cluster, pod)
				dbstate = newstate
				pod = newPod
			}

			if dbstate.IsStandby() {
				if dbstate.IsNormal() {
					podsBuilt = append(podsBuilt, pod)
				} else if !dbstate.IsConfigured() {
					podsToBuild = append(podsToBuild, pod)
				} else if dbstate.IsDisconnected() {
					continue
				}
			} else if dbstate.IsPending() {
				if !dbstate.IsConfigured() {
					podsToBuild = append(podsToBuild, pod)
				} else {
					// 实例已配置replconninfo, 重启为standby
					dbstate, _ = s.dbService.StartDBToStandby(&pod)
				}
			}
		}

		//通过basebackup，从Primary或者正常的Standby复制数据
		sourceIP := primaryIP
		if cluster.IsStandby() {
			remoutIpCount := len(cluster.GetValidSpec().RemoteIpList)
			sourceIP = cluster.GetValidSpec().RemoteIpList[remoutIpCount-1]
		} else {
			podsBuilt = s.getSyncPods(cluster, podsBuilt)
		}
		for len(podsToBuild) > 0 {
			s.Log.V(1).Info(fmt.Sprintf("[%s:%s]开始向%d个节点复制数据", cluster.Namespace, cluster.Name, len(podsToBuild)))
			if len(podsBuilt) > 0 {
				if len(podsBuilt) == 1 {
					sourceIP = podsBuilt[0].Status.PodIP
				} else {
					maxLsnPod := s.dbService.FindPodWithLargestLSN(podsBuilt, "")
					sourceIP = maxLsnPod.Status.PodIP
				}
			}
			targetPod := podsToBuild[0]
			if err := s.ensureStandby(cluster, targetPod, sourceIP, ipArray); err != nil {
				return err
			}
			//经过basebackup和配置后的节点，从目标节点数组移至源节点数组
			podsToBuild = podsToBuild[1:]
			podsBuilt = append(podsBuilt, targetPod)
		}
	}
	return nil
}

/*
数据恢复并配置Standby实例
方法参数：
	cluster：当前CR
	pod：当前Pod
	sourceIP：用于basebackup的源节点IP
	ipArr：需配置到连接信息的IP数组
返回值：
	当前Pod的实例状态
	错误信息
方法逻辑：
	从源节点进行basebackup
	如果成功则根据ipArr配置实例连接信息和其他参数
	如果失败则删除当前节点资源
*/
func (s *syncHandlerImpl) ensureStandby(cluster *opengaussv1.OpenGaussCluster, pod corev1.Pod, sourceIP string, ipArr []string) error {
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]从%s向%s复制数据", cluster.Namespace, cluster.Name, sourceIP, pod.Status.PodIP))
	state, success := s.dbService.BackupDB(&pod, sourceIP)
	if success {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]在Pod %s上完成数据复制", cluster.Namespace, cluster.Name, pod.Name))
		_, success = s.dbService.StartPending(&pod)
		if success {
			_, configured, _ := s.configDBInstance(cluster, &pod, ipArr, false, true, true)
			if configured {
				s.Log.V(1).Info(fmt.Sprintf("[%s:%s]位于Pod %s上的实例完成配置", cluster.Namespace, cluster.Name, pod.Name))
				return nil
			} else {
				return fmt.Errorf("[%s:%s]在Pod %s上配置数据库实例失败", cluster.Namespace, cluster.Name, pod.Name)
			}
		} else {
			return fmt.Errorf("[%s:%s]在Pod %s上启动数据库实例失败", cluster.Namespace, cluster.Name, pod.Name)
		}
	} else {
		if state.IsBackupFailed() {
			s.eventService.InstanceBasebackupFail(cluster, pod.Status.PodIP)
			s.Log.V(3).Info(fmt.Sprintf("[%s:%s]在Pod %s上复制数据失败，清理数据，稍后重试", cluster.Namespace, cluster.Name, pod.Name))
			if e := s.resourceService.CleanPodResource(cluster, pod.Status.PodIP); e != nil {
				return e
			}
		}
		return fmt.Errorf("[%s:%s]在Pod %s上复制数据失败", cluster.Namespace, cluster.Name, pod.Name)
	}
}

/*
清理集群实例
方法参数：
	cluster：当前CR
方法逻辑：
	将期望的IpList与实际的Pod的IP进行比较，如果不存在IP不在IpList中的Pod，则结束
	首先判断当前集群的Primary是否在CR.Spec.IpList中
		如果不在，需要从现有的Standby实例中，筛选出IpList中的实例，再从这些事例中选出一个作为新的Primary
		将选出的实例与现在的Primary做主从切换
	清理实例
		首先配置Primary，在连接信息中清理无关的IP
		遍历其他实例
			如果实例配置在IpList中，则更新连接信息
			否则删除实例所在Pod
*/
func (s *syncHandlerImpl) cleanupCluster(cluster *opengaussv1.OpenGaussCluster) error {
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]开始清理与规约中IP不符的数据库实例", cluster.Namespace, cluster.Name))
	pods, err := s.resourceService.FindPodsByCluster(cluster, false)
	if err != nil {
		return err
	}
	ipSet := cluster.GetValidSpec().IpSet()
	actualSet := utils.NewSet()
	for _, pod := range pods {
		actualSet.Add(pod.Status.PodIP)
	}
	_, missing := ipSet.DiffTo(actualSet)
	if len(missing) == 0 {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]数据库实例符合规约，无需清理", cluster.Namespace, cluster.Name))
		return nil
	} else {
		s.Log.V(1).Info(fmt.Sprintf("[%s:%s]规约中的IP是%s，实际使用的IP是%s，开始清理", cluster.Namespace, cluster.Name, ipSet.String(), actualSet.String()))
	}
	// 当前Primary不在IpList中，需要将已配置的一个Standby切换为新的Primary
	if cluster.IsPrimary() && !ipSet.Contains(cluster.Status.Primary) {
		configuredPods := make([]corev1.Pod, 0)
		for _, pod := range pods {
			if ipSet.Contains(pod.Status.PodIP) {
				configuredPods = append(configuredPods, pod)
			}
		}

		if cluster.IsPrimary() {
			configuredPods = s.getSyncPods(cluster, configuredPods)
		}
		//根据LSN选主
		newPrimaryPod := corev1.Pod{}
		if len(configuredPods) == 1 {
			newPrimaryPod = configuredPods[0]
		} else {
			newPrimaryPod = s.dbService.FindPodWithLargestLSN(configuredPods, "")
		}
		//主从切换
		if _, _, e := s.switchPrimary(cluster, cluster.Status.Primary, newPrimaryPod.Status.PodIP); e != nil {
			return e
		}
		cluster.Status.Primary = newPrimaryPod.Status.PodIP
	}

	//清理实例
	delCount := 0
	ipArray := ipSet.ToArray()

	//在Primary上清理连接信息
	if cluster.IsPrimary() {
		primaryPod, _ := s.resourceService.GetPod(cluster.Namespace, cluster.GetPodName(cluster.Status.Primary))
		_, ok, e := s.configDBInstance(cluster, primaryPod, ipArray, true, true, false)
		if !ok {
			s.Log.Error(e, fmt.Sprintf("[%s:%s]在Pod %s上配置数据实例，发生错误", cluster.Namespace, cluster.Name, primaryPod.Name))
			return e
		}
	}
	for _, pod := range pods {
		podIP := pod.Status.PodIP
		if podIP == cluster.Status.Primary {
			continue
		}
		if ipSet.Contains(podIP) {
			//更新连接信息
			_, configured, err := s.configDBInstance(cluster, &pod, ipArray, false, true, false)
			if !configured {
				s.Log.Error(err, fmt.Sprintf("[%s:%s]在Pod %s上配置数据实例，发生错误", cluster.Namespace, cluster.Name, pod.Name))
				continue
			}
		} else {
			//删除实例
			s.Log.V(1).Info(fmt.Sprintf("[%s:%s]删除Pod %s上的数据库实例", cluster.Namespace, cluster.Name, pod.Name))
			err := s.deletePod(cluster, podIP)
			if err != nil {
				s.Log.Error(err, fmt.Sprintf("[%s:%s]删除Pod %s上的数据库实例，发生错误", cluster.Namespace, cluster.Name, pod.Name))
				continue
			}
			delCount++
		}
	}
	//等待所有被删除的Pod被系统清理
	if delCount > 0 {
		result := s.resourceService.WaitPodsCleanup(cluster)
		if result {
			s.Log.V(1).Info(fmt.Sprintf("[%s:%s]数据库实例清理完成", cluster.Namespace, cluster.Name))
		} else {
			s.Log.V(1).Info(fmt.Sprintf("[%s:%s]数据库实例清理超时，稍后重试", cluster.Namespace, cluster.Name))
		}
	}

	return nil
}

/*
主从切换
方法参数：
	cluster：当前CR
	originPrimaryIP：原主IP
	newPrimaryIP：新主IP
返回值：
	原主状态
	新主状态
	错误信息
方法逻辑：
	确认原主与新主状态正常
	原主删除Label
	主从切换
	原主与新主更新Label
*/
func (s *syncHandlerImpl) switchPrimary(cluster *opengaussv1.OpenGaussCluster, originPrimaryIP, newPrimaryIP string) (utils.DBState, utils.DBState, error) {
	originPrimaryPod, err := s.resourceService.GetPod(cluster.Namespace, cluster.GetPodName(originPrimaryIP))
	if err != nil {
		s.Log.Error(err, fmt.Sprintf("[%s:%s]Failed to find primary pod with IP %s", cluster.Namespace, cluster.Name, originPrimaryIP))
		return utils.InitDBState(), utils.InitDBState(), err
	}
	newPrimaryPod, err := s.resourceService.GetPod(cluster.Namespace, cluster.GetPodName(newPrimaryIP))
	if err != nil {
		s.Log.Error(err, fmt.Sprintf("[%s:%s]Failed to find target pod with IP %s", cluster.Namespace, cluster.Name, newPrimaryIP))
		return utils.InitDBState(), utils.InitDBState(), err
	}
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]开始在%s和%s上的数据库实例间做主从切换", cluster.Namespace, cluster.Name, originPrimaryPod.Name, newPrimaryPod.Name))
	s.removeRoleLabelFromPod(cluster, originPrimaryIP)
	oldPrimaryState, newPrimaryState, err := s.dbService.SwitchPrimary(originPrimaryPod, newPrimaryPod)
	if err != nil {
		s.Log.Error(err, fmt.Sprintf("[%s:%s]在%s和%s之间进行主从切换，发生错误", cluster.Namespace, cluster.Name, originPrimaryPod.Name, newPrimaryPod.Name))
		return oldPrimaryState, newPrimaryState, err
	} else if !oldPrimaryState.IsStandby() || !newPrimaryState.IsPrimary() || !oldPrimaryState.IsNormal() || !newPrimaryState.IsNormal() {
		return oldPrimaryState, newPrimaryState, fmt.Errorf("[%s:%s]在%s和%s之间进行的主从切换未能成功", cluster.Namespace, cluster.Name, originPrimaryPod.Name, newPrimaryPod.Name)
	}
	s.addRoleLabelToPod(cluster, originPrimaryPod.Status.PodIP, false)
	s.addRoleLabelToPod(cluster, newPrimaryPod.Status.PodIP, true)
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]在%s和%s之间进行的主从切换已完成", cluster.Namespace, cluster.Name, originPrimaryPod.Name, newPrimaryPod.Name))
	s.eventService.InstanceSwitchover(cluster, originPrimaryIP, newPrimaryIP)
	return oldPrimaryState, newPrimaryState, nil
}

/*
更新数据库配置参数
方法参数：
	cluster：当前CR
方法逻辑：
	遍历集群的Pod
		更新实例的数据库配置
		如果更新的配置中有需要重启生效的参数，则重启数据库进程
*/
func (s *syncHandlerImpl) updateDBConfig(cluster *opengaussv1.OpenGaussCluster) error {
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]开始更新数据库配置，最新配置：%s", cluster.Namespace, cluster.Name, utils.MapToString(cluster.GetValidSpec().Config)))
	pods, err := s.resourceService.FindPodsByCluster(cluster, false)
	if err != nil {
		return err
	}
	changedConfig := cluster.ChangedConfig()
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]变更的配置有：%s", cluster.Namespace, cluster.Name, utils.MapToString(changedConfig)))
	for _, pod := range pods {
		configured, restartRequired, e := s.dbService.ConfigDBProperties(&pod, changedConfig)
		if e != nil {
			return e
		} else if !configured {
			return fmt.Errorf("[%s:%s]在Pod %s上更新配置失败", cluster.Namespace, cluster.Name, pod.Name)
		}
		if restartRequired {
			dbstate, e := s.dbService.CheckDBState(&pod)
			if e != nil {
				return e
			}
			if dbstate.IsPrimary() {
				s.dbService.RestartPrimary(&pod)
			} else if dbstate.IsStandby() {
				s.dbService.RestartStandby(&pod)
			}
		}
	}
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]数据库配置更新完成", cluster.Namespace, cluster.Name))
	return nil
}

/*
集群升级
方法参数：
	cluster：当前CR
方法逻辑：
	遍历集群Pod
		如果当前Pod的资源（镜像、CPU、内存、带宽）与CR.Spec不一致
			如果是Standby，则升级该实例Pod
			如果是Primary，则记录IP
	升级Primary
		重新查询集群Pod数组
		如果有Standby实例，则通过比较LSN选择新的Primary，做主从切换，然后升级原主Pod并启动为Standby
		如果没有，则直接升级Primary所在Pod并启动为Primary
*/
func (s *syncHandlerImpl) upgradeCluster(cluster *opengaussv1.OpenGaussCluster) error {
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]开始升级", cluster.Namespace, cluster.Name))
	pods, err := s.resourceService.FindPodsByCluster(cluster, false)
	if err != nil {
		return err
	}
	primaryToUpgrade := ""
	for _, pod := range pods {
		podIP := pod.Status.PodIP
		podName := pod.Name
		if dbstate, err := s.dbService.CheckDBState(&pod); err != nil {
			continue
		} else {
			if !s.resourceService.IsPodMatchWithSpec(cluster, pod) {
				if dbstate.IsStandby() {
					ok := s.upgradePod(cluster, podIP, false)
					if !ok {
						s.Log.Info(fmt.Sprintf("[%s:%s]Pod %s升级失败", cluster.Namespace, cluster.Name, podName))
						continue
					}
				} else if dbstate.IsPrimary() {
					primaryToUpgrade = pod.Status.PodIP
				}
			}
		}
	}
	if primaryToUpgrade != "" {
		standbyPods := make([]corev1.Pod, 0)
		//由于升级导致Pod重建，需要重新查询
		pods, err = s.resourceService.FindPodsByCluster(cluster, false)
		for _, pod := range pods {
			if pod.Status.PodIP != primaryToUpgrade {
				standbyPods = append(standbyPods, pod)
			}
		}
		if cluster.IsPrimary() {
			standbyPods = s.getSyncPods(cluster, standbyPods)
		}
		//升级Primary所在Pod
		restartToPrimary := false
		if len(standbyPods) > 0 {
			newPrimaryPod := corev1.Pod{}
			if len(standbyPods) == 1 {
				newPrimaryPod = standbyPods[0]
			} else {
				newPrimaryPod = s.dbService.FindPodWithLargestLSN(standbyPods, "")
			}
			if _, _, e := s.switchPrimary(cluster, primaryToUpgrade, newPrimaryPod.Status.PodIP); e != nil {
				return e
			}
		} else {
			restartToPrimary = true
		}
		if ok := s.upgradePod(cluster, primaryToUpgrade, restartToPrimary); !ok {
			return fmt.Errorf("[%s:%s]Pod %s升级失败", cluster.Namespace, cluster.Name, primaryToUpgrade)
		}
	}
	if s.isPreConfigMapExist(cluster) {
		if err := s.cleanupConfigMaps(cluster); err != nil {
			return err
		}
	}
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]升级完成", cluster.Namespace, cluster.Name))
	return nil
}

/*
升级实例
方法参数：
	cluster：当前CR
	podIP：需升级的Pod的IP
	startToPrimary：是否启动为Primary
返回值：
	实例状态
	是否升级成功
方法逻辑：
	删除需升级的Pod
	根据当前CR.Spec重建Pod
	等待Pod进入running状态
	配置数据库参数
	根据startToPrimary启动数据库为Primary或Standby
	移除维护标记文件，添加匹配的Label
*/
func (s *syncHandlerImpl) upgradePod(cluster *opengaussv1.OpenGaussCluster, podIP string, startToPrimary bool) bool {
	podName := cluster.GetPodName(podIP)
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]升级Pod %s", cluster.Namespace, cluster.Name, podName))
	node := ""
	for _, entry := range cluster.GetValidSpec().IpList {
		if entry.Ip == podIP {
			node = entry.NodeName
			break
		}
	}
	pod, err := s.resourceService.GetPod(cluster.Namespace, podName)
	if err != nil {
		return false
	}
	if e := s.resourceService.DeletePod(pod); e != nil {
		return false
	}
	//确保Pod删除完成
	deleted := s.resourceService.WaitPodCleanup(cluster, podName)
	if !deleted {
		return false
	}

	_, _, err = s.resourceService.EnsurePodResource(cluster, opengaussv1.IpNodeEntry{Ip: podIP, NodeName: node})
	if err != nil {
		return false
	}
	//确保Pod启动
	newPod, started := s.resourceService.WaitPodRunning(cluster, podName)
	if !started {
		return false
	}

	//等待30秒，以确保Pod的liveness探针启动，否则K8S将会在方法末尾删除维护标记文件后重启容器
	time.Sleep(time.Second * 30)

	ok := false
	restart := false
	//如果有参数改变，需要配置
	if cluster.IsDBConfigChange() {
		configured, restartRequired, e := s.dbService.ConfigDBProperties(&newPod, cluster.ChangedConfig())
		if !configured || e != nil {
			return false
		}
		restart = restartRequired
	}
	//启动实例
	if startToPrimary {
		if restart {
			_, ok = s.dbService.RestartPrimary(&newPod)
		} else {
			_, ok = s.dbService.StartPrimary(&newPod)
		}
	} else {
		if restart {
			_, ok = s.dbService.RestartStandby(&newPod)
		} else {
			_, ok = s.dbService.StartStandby(&newPod)
		}
	}

	//移除维护标记文件，添加Label
	_, ok = s.dbService.RemoveMaintenanceFlag(&newPod)
	s.addRoleLabelToPod(cluster, podIP, startToPrimary)
	s.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod %s升级完成", cluster.Namespace, cluster.Name, pod.Name))
	s.eventService.InstanceUpgrade(cluster, podIP)
	return ok
}

/*
为Pod添加角色Label
方法参数：
	cluster：当前CR
	ip：Pod IP
	isPrimary：是否是Primary实例
*/
func (s *syncHandlerImpl) addRoleLabelToPod(cluster *opengaussv1.OpenGaussCluster, ip string, isPrimary bool) error {
	podName := cluster.GetPodName(ip)
	return s.resourceService.AddRoleLabelToPod(cluster.Namespace, podName, isPrimary)
}

/*
删除Pod角色Label
方法参数：
	cluster：当前CR
	ip： Pod IP
*/
func (s *syncHandlerImpl) removeRoleLabelFromPod(cluster *opengaussv1.OpenGaussCluster, ip string) error {
	podName := cluster.GetPodName(ip)
	return s.resourceService.RemoveRoleLabelFromPod(cluster.Namespace, podName)
}

/*
删除Pod
方法参数：
	cluster：当前CR
	ip： Pod IP
*/
func (s *syncHandlerImpl) deletePod(cluster *opengaussv1.OpenGaussCluster, ip string) error {
	podName := cluster.GetPodName(ip)
	pod, e := s.resourceService.GetPod(cluster.Namespace, podName)
	if e != nil {
		return e
	}
	if err := s.resourceService.DeletePod(pod); err != nil {
		return err
	}
	s.eventService.InstanceDelete(cluster, ip)
	return nil
}

/*
配置数据库实例
方法参数：
	cluster：当前集群
	pod：当前实例
	ipArray：集群实例的IP数组
	primary：是否配置为主库
	start：是否启动数据库
返回值：
	数据库状态
	是否配置完成
	错误信息
*/
func (s *syncHandlerImpl) configDBInstance(cluster *opengaussv1.OpenGaussCluster, pod *corev1.Pod, ipArray []string, primary, start, logEvent bool) (utils.DBState, bool, error) {
	dbstate, configured, err := s.dbService.ConfigDB(pod, ipArray, cluster.GetValidSpec().RemoteIpList, primary, start, cluster.GetValidSpec().Config)
	if !configured {
		s.eventService.InstanceConfigFail(cluster, pod.Status.PodIP, err)
	} else if logEvent {
		if primary {
			s.eventService.InstanceSetPrimary(cluster, pod.Status.PodIP)
		} else {
			s.eventService.InstanceSetStandby(cluster, pod.Status.PodIP)
		}
	}
	return dbstate, configured, err
}
func (s *syncHandlerImpl) upgradeRequired(cluster *opengaussv1.OpenGaussCluster) bool {
	if cluster.IsUpgrade() {
		return true
	}
	if !cluster.IsNew() && cluster.Status.Spec.Schedule.MostAvailableTimeout == 0 {
		return true
	}
	return s.isPreConfigMapExist(cluster)
}
func (s *syncHandlerImpl) isPreConfigMapExist(cluster *opengaussv1.OpenGaussCluster) bool {
	cmPreviousVersion := fmt.Sprintf("%s-init-cm", cluster.Name)
	if _, err := s.resourceService.GetConfigMap(cluster.Namespace, cmPreviousVersion); err == nil {
		return true
	}
	return false
}
func (s *syncHandlerImpl) cleanupConfigMaps(cluster *opengaussv1.OpenGaussCluster) error {
	types := []string{"db", "log", "sh", "init"}
	for _, t := range types {
		cmName := fmt.Sprintf("%s-%s-cm", cluster.Name, t)
		if e := s.resourceService.DeleteConfigMap(cluster.Namespace, cmName); e != nil {
			return e
		}
	}
	return nil
}
