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

package service

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	opengaussv1 "opengauss-operator/api/v1"
	"opengauss-operator/utils"
)

const (
	REASON_CREATE                 = "Creating"
	REASON_UPDATE                 = "Updating"
	REASON_RECOVER                = "Recorvering"
	REASON_RESTORE                = "restoring"
	REASON_MAINTAIN_START         = "MaintainStart"
	REASON_MAINTAIN_COMPLETE      = "MaintainComplete"
	REASON_READY                  = "Ready"
	REASON_VALIDATE_FAILED        = "ValidateFail"
	REASON_VALIDATE_FIX           = "ValidateFix"
	REASON_SET_PRIMARY            = "SetPrimary"
	REASON_SET_STANDBY            = "SetStandby"
	REASON_CONFIGDB_FAIL          = "ConfigDBFail"
	REASON_RESTORE_FAIL           = "RestoreFail"
	REASON_BASEBACKUP_FAIL        = "BasebackupFail"
	REASON_FIX_STANDBY_COMPLETE   = "FixStandbyComplete"
	REASON_FIX_STANDBY_FAIL       = "FixStandbyFail"
	REASON_SWITCH_PRIMARY         = "SwitchPrimary"
	REASON_DELETE_INSTANCE        = "DeleteInstance"
	REASON_UPGRADE_INSTANCE       = "UpgradeInstance"
	REASON_INSTANCE_START_TIMEOUT = "InstanceStartTimeout"
	REASON_SET_MOST_AVAILABLE     = "SetMostAvailable"
	REASON_CLUSTER_FAILED         = "Failed"
	DEFAULT_ERROR_MESSAGE         = "unknown"
)

type EventService interface {
	ClusterCreate(cluster *opengaussv1.OpenGaussCluster)
	ClusterUpdate(cluster *opengaussv1.OpenGaussCluster)
	ClusterRecover(cluster *opengaussv1.OpenGaussCluster)
	ClusterRestore(cluster *opengaussv1.OpenGaussCluster)
	ClusterReady(cluster *opengaussv1.OpenGaussCluster)
	ClusterMaintainStart(cluster *opengaussv1.OpenGaussCluster)
	ClusterMaintainComplete(cluster *opengaussv1.OpenGaussCluster)
	ClusterValidateFailed(cluster *opengaussv1.OpenGaussCluster, err error)
	ClusterValidateFixed(cluster *opengaussv1.OpenGaussCluster)
	ClusterFailed(cluster *opengaussv1.OpenGaussCluster, err error)
	InstanceSetPrimary(cluster *opengaussv1.OpenGaussCluster, ip string)
	InstanceSetStandby(cluster *opengaussv1.OpenGaussCluster, ip string)
	InstanceSwitchover(cluster *opengaussv1.OpenGaussCluster, source, target string)
	InstanceConfigFail(cluster *opengaussv1.OpenGaussCluster, ip string, err error)
	InstanceRestoreFail(cluster *opengaussv1.OpenGaussCluster, ip string)
	InstanceBasebackupFail(cluster *opengaussv1.OpenGaussCluster, ip string)
	FixStandbyComplete(cluster *opengaussv1.OpenGaussCluster, ip, state string)
	FixStandbyFail(cluster *opengaussv1.OpenGaussCluster, ip, state string)
	InstanceDelete(cluster *opengaussv1.OpenGaussCluster, ip string)
	InstanceUpgrade(cluster *opengaussv1.OpenGaussCluster, ip string)
	InstanceTimeout(cluster *opengaussv1.OpenGaussCluster, ip string)
	ConfigureMostAvailable(cluster *opengaussv1.OpenGaussCluster, enable bool)
}

type eventService struct {
	eventsCli record.EventRecorder
}

func NewEventService(eventRecorder record.EventRecorder) EventService {
	return &eventService{
		eventsCli: eventRecorder,
	}
}

func (e *eventService) ClusterCreate(cluster *opengaussv1.OpenGaussCluster) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_CREATE, fmt.Sprintf("%s 创建集群", time.Now().Format(utils.TIME_FORMAT)))
}

func (e *eventService) ClusterUpdate(cluster *opengaussv1.OpenGaussCluster) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_UPDATE, fmt.Sprintf("%s 更新集群", time.Now().Format(utils.TIME_FORMAT)))
}

func (e *eventService) ClusterRecover(cluster *opengaussv1.OpenGaussCluster) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeWarning, REASON_RECOVER, fmt.Sprintf("%s 恢复集群", time.Now().Format(utils.TIME_FORMAT)))
}

func (e *eventService) ClusterRestore(cluster *opengaussv1.OpenGaussCluster) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_RESTORE, fmt.Sprintf("%s 从文件%s恢复数据", time.Now().Format(utils.TIME_FORMAT), cluster.GetValidSpec().RestoreFile))
}

func (e *eventService) ClusterReady(cluster *opengaussv1.OpenGaussCluster) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_READY, fmt.Sprintf("%s 集群正常", time.Now().Format(utils.TIME_FORMAT)))
}

func (e *eventService) ClusterMaintainStart(cluster *opengaussv1.OpenGaussCluster) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_MAINTAIN_START, fmt.Sprintf("%s 开启维护模式", time.Now().Format(utils.TIME_FORMAT)))
}

func (e *eventService) ClusterMaintainComplete(cluster *opengaussv1.OpenGaussCluster) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_MAINTAIN_COMPLETE, fmt.Sprintf("%s 维护模式结束", time.Now().Format(utils.TIME_FORMAT)))
}
func (e *eventService) ClusterFailed(cluster *opengaussv1.OpenGaussCluster, err error) {
	message := DEFAULT_ERROR_MESSAGE
	if err != nil {
		message = err.Error()
	}
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeWarning, REASON_CLUSTER_FAILED, fmt.Sprintf("%s 故障:%s，等待人工处理", time.Now().Format(utils.TIME_FORMAT), message))
}

func (e *eventService) ClusterValidateFailed(cluster *opengaussv1.OpenGaussCluster, err error) {
	message := DEFAULT_ERROR_MESSAGE
	if err != nil {
		message = err.Error()
	}
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeWarning, REASON_VALIDATE_FAILED, fmt.Sprintf("%s 校验错误:%s", time.Now().Format(utils.TIME_FORMAT), message))
}

func (e *eventService) ClusterValidateFixed(cluster *opengaussv1.OpenGaussCluster) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_VALIDATE_FIX, fmt.Sprintf("%s 校验更正", time.Now().Format(utils.TIME_FORMAT)))
}

func (e *eventService) InstanceSetPrimary(cluster *opengaussv1.OpenGaussCluster, ip string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_SET_PRIMARY, fmt.Sprintf("%s 实例%s设置为主库", time.Now().Format(utils.TIME_FORMAT), ip))
}

func (e *eventService) InstanceSetStandby(cluster *opengaussv1.OpenGaussCluster, ip string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_SET_STANDBY, fmt.Sprintf("%s 实例%s设置为从库", time.Now().Format(utils.TIME_FORMAT), ip))
}

func (e *eventService) InstanceSwitchover(cluster *opengaussv1.OpenGaussCluster, source, target string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_SWITCH_PRIMARY, fmt.Sprintf("%s 主库从%s切换至%s", time.Now().Format(utils.TIME_FORMAT), source, target))
}

func (e *eventService) InstanceConfigFail(cluster *opengaussv1.OpenGaussCluster, ip string, err error) {
	message := DEFAULT_ERROR_MESSAGE
	if err != nil {
		message = err.Error()
	}
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeWarning, REASON_CONFIGDB_FAIL, fmt.Sprintf("%s 实例%s配置错误:%s", time.Now().Format(utils.TIME_FORMAT), ip, message))
}

func (e *eventService) InstanceRestoreFail(cluster *opengaussv1.OpenGaussCluster, ip string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeWarning, REASON_RESTORE_FAIL, fmt.Sprintf("%s 实例%s恢复数据失败，将于清理后重试", time.Now().Format(utils.TIME_FORMAT), ip))
}

func (e *eventService) InstanceBasebackupFail(cluster *opengaussv1.OpenGaussCluster, ip string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeWarning, REASON_BASEBACKUP_FAIL, fmt.Sprintf("%s 实例%s复制数据失败，将于清理后重试", time.Now().Format(utils.TIME_FORMAT), ip))
}

func (e *eventService) FixStandbyComplete(cluster *opengaussv1.OpenGaussCluster, ip, state string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_FIX_STANDBY_COMPLETE, fmt.Sprintf("%s 重启修复实例%s完成，实例状态是%s", time.Now().Format(utils.TIME_FORMAT), ip, state))
}

func (e *eventService) FixStandbyFail(cluster *opengaussv1.OpenGaussCluster, ip, state string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeWarning, REASON_FIX_STANDBY_FAIL, fmt.Sprintf("%s 重启修复实例%s失败，实例状态是%s", time.Now().Format(utils.TIME_FORMAT), ip, state))
}

func (e *eventService) InstanceDelete(cluster *opengaussv1.OpenGaussCluster, ip string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_DELETE_INSTANCE, fmt.Sprintf("%s 删除实例%s", time.Now().Format(utils.TIME_FORMAT), ip))
}

func (e *eventService) InstanceUpgrade(cluster *opengaussv1.OpenGaussCluster, ip string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_UPGRADE_INSTANCE, fmt.Sprintf("%s 升级实例%s", time.Now().Format(utils.TIME_FORMAT), ip))
}

func (e *eventService) InstanceTimeout(cluster *opengaussv1.OpenGaussCluster, ip string) {
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeWarning, REASON_INSTANCE_START_TIMEOUT, fmt.Sprintf("%s 实例%s启动超时", time.Now().Format(utils.TIME_FORMAT), ip))
}

func (e *eventService) ConfigureMostAvailable(cluster *opengaussv1.OpenGaussCluster, enable bool) {
	val := PARAM_VALUE_OFF
	if enable {
		val = PARAM_VALUE_ON
	}
	e.eventsCli.Event(cluster.DeepCopyObject(), corev1.EventTypeNormal, REASON_SET_MOST_AVAILABLE, fmt.Sprintf("%s 设置最大可用为\"%s\"", time.Now().Format(utils.TIME_FORMAT), val))
}
