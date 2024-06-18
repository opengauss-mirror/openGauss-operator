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
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	opengaussv1 "opengauss-operator/api/v1"
	"opengauss-operator/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	OPENGAUSS_APP_KEY             = "app.kubernetes.io/app"
	OPENGAUSS_APP_VAL             = "opengauss"
	OPENGAUSS_CLUSTER_KEY         = "opengauss.cluster"
	OPENGAUSS_ROLE_KEY            = "opengauss.role"
	CONFIGMAP_TYPE_DB             = "db"
	CONFIGMAP_TYPE_LOG            = "log"
	CONFIGMAP_TYPE_SHELL          = "sh"
	CONFIGMAP_TYPE_PID            = "pid"
	CONFIGMAP_TYPE_INIT           = "init"
	BANDWIDTH_INGRESS_KEY         = "kubernetes.io/ingress-bandwidth"
	BANDWIDTH_EGRESS_KEY          = "kubernetes.io/egress-bandwidth"
	KUBEOVN_BANDWIDTH_INGRESS_KEY = "ovn.kubernetes.io/ingress_rate"
	KUBEOVN_BANDWIDTH_EGRESS_KEY  = "ovn.kubernetes.io/egress_rate"
	OG_DB_ROLE_PRIMARY            = "primary"
	OG_DB_ROLE_STANDBY            = "standby"
	OG_CLUSTER_CONFIGMAP_NAME     = "opengauss-cluster-scripts"
	OG_SCRIPT_CONFIGMAP_NAME      = "opengauss-management-scripts"
	OG_FILEBEAT_CONFIGMAP_NAME    = "opengauss-filebeat-config"
	OG_OG_PASSWORD                = "OG_PASSWORD"
	OG_MY_POD_IP                  = "MY_POD_IP"
	SIDECAR_CR_NAME               = "CR_NAME"
	SIDECAR_MY_POD_IP             = "MY_POD_IP"
	ADMIN_PVC_NAME                = "pvc-admin"
	POD_FAILEDATTACHVOLUME_EVENT  = "failedattachvolume"
	POD_FAILEDMOUNT_EVENT         = "failedmount"
)

type IResourceService interface {
	EnsureConfigMaps(cluster *opengaussv1.OpenGaussCluster) error
	EnsureServices(cluster *opengaussv1.OpenGaussCluster) error
	EnsurePodResource(cluster *opengaussv1.OpenGaussCluster, entry opengaussv1.IpNodeEntry) (bool, bool, error)
	EnsureSecret(cluster *opengaussv1.OpenGaussCluster) error
	FindPodsByCluster(cluster *opengaussv1.OpenGaussCluster, sort bool) ([]corev1.Pod, error)
	CheckClusterArtifacts(cluster *opengaussv1.OpenGaussCluster) error
	GetPod(namespace, name string) (*corev1.Pod, error)
	GetConfigMap(namespace, name string) (*corev1.ConfigMap, error)
	DeleteConfigMap(namespace, name string) error
	DeletePod(pod *corev1.Pod) error
	ForceDeletePod(pod *corev1.Pod) error
	IsPodReady(pod corev1.Pod) bool
	IsPodMatchWithSpec(cluster *opengaussv1.OpenGaussCluster, pod corev1.Pod) bool
	GetCluster(namespace, name string) (*opengaussv1.OpenGaussCluster, error)
	CleanPodResource(cluster *opengaussv1.OpenGaussCluster, ip string) error
	AddRoleLabelToPod(namespace, podName string, primary bool) error
	RemoveRoleLabelFromPod(namespace string, pod *corev1.Pod) error
	WaitPodsRunning(cluster *opengaussv1.OpenGaussCluster) ([]corev1.Pod, bool)
	WaitPodsCleanup(cluster *opengaussv1.OpenGaussCluster) bool
	WaitPodRunning(cluster *opengaussv1.OpenGaussCluster, podName string) (corev1.Pod, bool)
	WaitPodCleanup(cluster *opengaussv1.OpenGaussCluster, podName string) bool
	SetContainerEnvs(cluster *opengaussv1.OpenGaussCluster, pod *corev1.Pod)
	SetContainerCustomizedenv(cluster *opengaussv1.OpenGaussCluster, pod *corev1.Pod)
	ClusterPodsIsHaveAdminPvc(cluster *opengaussv1.OpenGaussCluster) bool
	PodIsHaveAdminPvc(pod corev1.Pod) bool
	IsIncludeFailMountEvents(namespace, clusterName, podName string) (bool, corev1.Event)
	IsExtendIpMatch(cluster *opengaussv1.OpenGaussCluster, pod corev1.Pod, ipExtendIpMap map[string]string) bool
	ClusterPodsIsExtendIpMatch(cluster *opengaussv1.OpenGaussCluster) bool
}

type resourceService struct {
	client       client.Client
	Log          logr.Logger
	KubeClient   *kubernetes.Clientset
	eventService EventService
	dbService    IDBService
}

func NewResourceService(client client.Client, logger logr.Logger, eventRecorder record.EventRecorder) IResourceService {
	resourceService := &resourceService{client: client, Log: logger, eventService: NewEventService(eventRecorder)}
	resourceService.KubeClient = kubernetes.NewForConfigOrDie(ctrl.GetConfigOrDie())
	resourceService.dbService = NewDBService(client, logger)
	return resourceService
}

/*
维护ConfigMap
*/
func (r *resourceService) EnsureConfigMaps(cluster *opengaussv1.OpenGaussCluster) error {
	if e := r.ensureClusterConfigMap(cluster); e != nil {
		return e
	}
	if e := r.ensureScriptConfigMap(cluster); e != nil {
		return e
	}
	if e := r.ensureFilebeatConfigMap(cluster); e != nil {
		return e
	}
	return nil
}

func (r *resourceService) ensureClusterConfigMap(cluster *opengaussv1.OpenGaussCluster) error {
	cmName := cluster.GetConfigMapName()
	cm, exist, err := r.findConfigMap(cluster.Namespace, cluster.Name, cmName)
	if err != nil {
		return err
	}

	if !exist {
		newConfigMap, err := r.newClusterConfigMap(cluster)
		if err != nil {
			return err
		}
		err = r.client.Create(context.TODO(), newConfigMap)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]创建ConfigMap %s，发生错误", cluster.Namespace, cluster.Name, cmName))
			return err
		}
	} else if cluster.Status.Spec.Schedule.ProcessTimeout == 0 {
		cm, _ = r.newClusterConfigMap(cluster)
		r.client.Update(context.TODO(), cm)
		time.Sleep(30)
	} else {
		//判断初始化脚本是否包含${NEED_INIT},不包含，代表存量数据
		for k, v := range cm.Data {
			if k == "k8s-initenv.sh" {
				if !strings.Contains(v, "${NEED_INIT}") {
					r.Log.Info(fmt.Sprintf("[%s:%s] 存量集群，更新configmap", cluster.Namespace, cluster.Name))
					cm, _ = r.newClusterConfigMap(cluster)
					r.client.Update(context.TODO(), cm)
					time.Sleep(30)

				}
			}
			if k == "og.entrypoint.sh" {
				if !strings.Contains(v, "postmaster.pid.lock") {
					r.Log.Info(fmt.Sprintf("[%s:%s] 存量集群，更新configmap->og.entrypoint.sh", cluster.Namespace, cluster.Name))
					cm, _ = r.newClusterConfigMap(cluster)
					r.client.Update(context.TODO(), cm)
					time.Sleep(30)

				}
			}
		}

	}

	return nil
}

func (r *resourceService) ensureScriptConfigMap(cluster *opengaussv1.OpenGaussCluster) error {
	cmName := cluster.GetValidSpec().ScriptConfig
	_, exist, err := r.findConfigMap(cluster.Namespace, cluster.Name, cmName)
	if err != nil {
		return err
	}

	if !exist {
		newConfigMap, err := r.newScriptConfigMap(cluster)
		if err != nil {
			return err
		}
		err = r.client.Create(context.TODO(), newConfigMap)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]创建ConfigMap %s，发生错误", cluster.Namespace, cluster.Name, cmName))
			return err
		}
	}

	return nil
}

func (r *resourceService) ensureFilebeatConfigMap(cluster *opengaussv1.OpenGaussCluster) error {
	cmName := cluster.GetValidSpec().FilebeatConfig
	_, exist, err := r.findConfigMap(cluster.Namespace, cluster.Name, cmName)
	if err != nil {
		return err
	}
	if !exist {
		newConfigMap, err := r.newFilebeatConfigMap(cluster.Namespace, cmName)
		if err != nil {
			return err
		}
		err = r.client.Create(context.TODO(), newConfigMap)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]创建ConfigMap %s，发生错误", cluster.Namespace, cluster.Name, cmName))
			return err
		}
	}
	return nil
}

/*
维护集群的Service
*/
func (r *resourceService) EnsureServices(cluster *opengaussv1.OpenGaussCluster) error {
	if error := r.ensureService(cluster, false); error != nil {
		return error
	}
	if error := r.ensureService(cluster, true); error != nil {
		return error
	}
	return nil
}

/*
维护集群的Pod、PVC和PV
方法参数：
	cluster：当前CR
	entry：需要维护的实例
返回值：
	是否新建了PVC
	是否新建了Pod
	错误信息
*/
func (r *resourceService) EnsurePodResource(cluster *opengaussv1.OpenGaussCluster, entry opengaussv1.IpNodeEntry) (bool, bool, error) {
	if cluster.IsHostpathEnable() {
		r.ensurePVs(cluster, entry)
	}
	pvcCreated, err := r.ensurePVCs(cluster, entry.Ip)
	if err != nil {
		return pvcCreated, false, err
	}

	podCreated, err := r.ensurePod(cluster, entry, pvcCreated)
	if err != nil {
		return pvcCreated, podCreated, err
	}
	return pvcCreated, podCreated, nil
}

/*
维护Pod
方法参数：
	cluster：当前集群
	entry：实例的IP和node信息
返回值：
	是否新建Pod
	错误信息
方法逻辑：
	如果Pod不存在，新建Pod
	如果Pod存在但状态不正常（Pending或其他非running状态），删除Pod和相关的PVC/PV资源，新建Pod
*/
func (r *resourceService) ensurePod(cluster *opengaussv1.OpenGaussCluster, entry opengaussv1.IpNodeEntry, pvcNewCreate bool) (bool, error) {
	exist := true
	podName := cluster.GetPodName(entry.Ip)
	existPod, err := r.GetPod(cluster.Namespace, podName)
	if err != nil {
		if errors.IsNotFound(err) {
			exist = false
		} else {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询名为%s的Pod，发生错误.", cluster.Namespace, cluster.Name, cluster.GetPodName(entry.Ip)))
			return false, err
		}
	}
	r.Log.Info(fmt.Sprintf("pvcNewCreate value is [%v]", pvcNewCreate))

	if exist && !r.IsPodReady(*existPod) {
		isPending := false
		if existPod.Status.Phase == corev1.PodRunning {
			if !r.IsDbContainerPortReady(*existPod) {
				// pod本身是running，仅db进程不存在，尝试重启为pending
				_, isPending = r.dbService.StartPending(existPod)
			} else {
				isPending = true
			}

		}
		if !isPending {
			//如果pod已存在，但不是ready状态，需要判断不是ready状态的原因，是否因为failmout而创建失败（通过获取pod的event时间）
			//event事件中包含文件头损坏/failmount类型的错误，则不清理当前pod
			if isInclued, event := r.IsIncludeFailMountEvents(cluster.Namespace, cluster.Name, existPod.Name); isInclued {
				r.eventService.InstanceStartFail(cluster, entry.Ip, event.Reason+","+event.Message)
			} else {
				r.Log.Info(fmt.Sprintf("[%s:%s]Pod %s未处于运行状态或无法访问，将删除重建", cluster.Namespace, cluster.Name, existPod.Name))
				r.CleanPodResource(cluster, entry.Ip)
				if entry.Ip != cluster.Status.Primary {
					pvcNewCreate, _ = r.ensurePVCs(cluster, entry.Ip)
					if cluster.IsHostpathEnable() {
						r.ensurePVs(cluster, entry)
					}
				}
				exist = false
			}
		}

	}
	r.Log.Info(fmt.Sprintf("pvcNewCreate value is [%v]", pvcNewCreate))

	if exist {
		if cluster.IsLabelChange() {
			err := r.updatePodLabel(existPod, cluster.GetValidSpec().Labels, cluster.Status.Spec.Labels, false)
			if err != nil {
				return false, err
			}
		}
		if cluster.IsAnnotationChange() {
			err := r.updatePodAnnotations(existPod, cluster, entry, false)
			if err != nil {
				return false, err
			}
		}
		//如果带宽发生变化，且网络插件为kube-ovn，直接更新pod的注解
		if cluster.IsBandwidthChange() {
			if opengaussv1.NETWORK_KUBE_OVN == cluster.GetValidSpec().NetworkClass {
				r.setPodBandwidth(existPod, cluster)
				return false, r.client.Update(context.TODO(), existPod)
			}
			return false, nil
		}
		return false, nil
	}

	pod, err := r.newPod(cluster, entry, pvcNewCreate)
	if err != nil {
		return false, err
	}
	err = r.client.Create(context.TODO(), pod)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]创建名为%s的Pod，发生错误", cluster.Namespace, cluster.Name, cluster.GetPodName(entry.Ip)))
		return false, err
	}
	return true, nil
}

/*
删除Pod和相关资源
方法参数：
	cluster：当前CR
	ip：Pod IP
方法逻辑：
	删除当前Pod
	删除所有相关存储资源
*/
func (r *resourceService) CleanPodResource(cluster *opengaussv1.OpenGaussCluster, ip string) error {
	existPod, _ := r.GetPod(cluster.Namespace, cluster.GetPodName(ip))
	if e := r.DeletePod(existPod); e != nil {
		return e
	} else {
		if deleted := r.WaitPodCleanup(cluster, cluster.GetPodName(ip)); !deleted {
			r.Log.Info(fmt.Sprintf("[%s:%s未能正常删除pod %s，执行强制删除操作]", cluster.Namespace, cluster.Name, existPod.Name))
			r.ForceDeletePod(existPod)
			if forceDeleted := r.WaitPodCleanup(cluster, cluster.GetPodName(ip)); !forceDeleted {
				return fmt.Errorf("[%s:%s]未能删除Pod %s", cluster.Namespace, cluster.Name, cluster.GetPodName(ip))
			}
		}
	}
	return nil
}

/*
清理存储资源
方法参数：
	cluster：当前CR
	ip： Pod IP
	pvcType：存储资源类型（data/log/admin）
*/
func (r *resourceService) cleanStorage(cluster *opengaussv1.OpenGaussCluster, ip, pvcType string) error {
	pvcName := cluster.GetPVCName(ip, pvcType)
	pvc, _ := r.getPVC(cluster.Namespace, pvcName)
	if e := r.deletePVC(pvc); e != nil {
		return e
	} else {
		if deleted := r.WaitPVCCleanup(cluster, pvcName); !deleted {
			return fmt.Errorf("[%s:%s]未能删除PVC %s", cluster.Namespace, cluster.Name, pvcName)
		} else if cluster.IsHostpathEnable() {
			pvName := cluster.GetPVCName(ip, pvcType)
			pv, _ := r.getPV(pvName)
			if err := r.deletePV(pv); err != nil {
				return err
			} else {
				if del := r.WaitPVleanup(cluster, pvName); !del {
					return fmt.Errorf("[%s:%s]未能删除PV %s", cluster.Namespace, cluster.Name, pvName)
				}
			}
		}
	}
	return nil
}

/*
维护一个节点的PV
*/
func (r *resourceService) ensurePVs(cluster *opengaussv1.OpenGaussCluster, entry opengaussv1.IpNodeEntry) error {
	if err := r.ensurePV(cluster, entry, VOLUME_TYPE_DATA, cluster.GetValidSpec().Storage); err != nil {
		return err
	}
	if err := r.ensurePV(cluster, entry, VOLUME_TYPE_LOG, cluster.GetValidSpec().SidecarStorage); err != nil {
		return err
	}
	return nil
}

/*
维护一个PV
方法参数：
	cluster：当前CR
	entry：一个节点的ip和node名称
	pvType：pv的类型（data/log/admin）
	request：要求的存储容量
*/
func (r *resourceService) ensurePV(cluster *opengaussv1.OpenGaussCluster, entry opengaussv1.IpNodeEntry, pvType, request string) error {
	pvName := cluster.GetPVName(entry.Ip, pvType)

	exist := true
	existPv, err := r.getPV(pvName)
	if err != nil {
		if errors.IsNotFound(err) {
			exist = false
		} else {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询名为%s的PV，发生错误", cluster.Namespace, cluster.Name, pvName))
			return err
		}
	}

	if !exist {
		r.Log.V(1).Info(fmt.Sprintf("[%s:%s]创建PV %s", cluster.Namespace, cluster.Name, pvName))
		pv, err := r.newPV(cluster, entry, pvType, request)
		if err != nil {
			return err
		}
		//PV在PVC删除后被释放，状态为release，但无法被查询到
		//当新建PV发生冲突时，通过Update()可修改PV状态为可用，然后可以被查询到和被PVC绑定
		err = r.client.Create(context.TODO(), pv)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				r.Log.V(1).Info(fmt.Sprintf("[%s:%s]PV %s已存在，尝试刷新", cluster.Namespace, cluster.Name, pvName))
				err = r.client.Update(context.TODO(), pv)
				if err != nil {
					r.Log.Error(err, fmt.Sprintf("[%s:%s]刷新PV %s，发生错误", cluster.Namespace, cluster.Name, pvName))
					return err
				}
			} else {
				r.Log.Error(err, fmt.Sprintf("[%s:%s]创建PV %s，发生错误", cluster.Namespace, cluster.Name, pvName))
				return err
			}
		} else {
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]创建PV %s完成", cluster.Namespace, cluster.Name, pvName))
		}
	} else if needUpdatePV(existPv, request) {
		err = r.client.Update(context.TODO(), existPv)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]更新PV %s，发生错误", cluster.Namespace, cluster.Name, pvName))
			return err
		}
	}

	return nil
}

/*
维护一个节点的PVC
方法参数：
	cluster：当前CR
	ip： Pod IP
返回值：
	是否新建了PVC
	错误信息
*/
func (r *resourceService) ensurePVCs(cluster *opengaussv1.OpenGaussCluster, ip string) (bool, error) {
	pvcCreated, err := r.ensurePVC(cluster, ip, VOLUME_TYPE_DATA, cluster.GetValidSpec().Storage)
	if err != nil {
		return pvcCreated, err
	}
	// ignore if sidecar pvc exist or not
	_, err = r.ensurePVC(cluster, ip, VOLUME_TYPE_LOG, cluster.GetValidSpec().SidecarStorage)
	if err != nil {
		return pvcCreated, err
	}
	return pvcCreated, nil
}

/*
维护一个PVC
方法参数：
	cluster：当前CR
	ip：Pod IP
	pvcType：PVC的类型（data/log/admin）
	request：要求的存储容量
方法参数：
	是否新建了PVC
	错误信息
*/
func (r *resourceService) ensurePVC(cluster *opengaussv1.OpenGaussCluster, ip, pvcType, request string) (bool, error) {
	pvcName := cluster.GetPVCName(ip, pvcType)

	exist := true
	existPvc, err := r.getPVC(cluster.Namespace, pvcName)
	if err != nil {
		if errors.IsNotFound(err) {
			exist = false
		} else {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询名为%s的PVC，发生错误", cluster.Namespace, cluster.Name, pvcName))
			return false, err
		}
	}

	created := false
	if !exist {
		r.Log.V(1).Info(fmt.Sprintf("[%s:%s]创建PVC %s", cluster.Namespace, cluster.Name, pvcName))
		pvc, err := r.newPVC(cluster, ip, pvcType, request)
		if err != nil {
			return created, err
		}
		err = r.client.Create(context.TODO(), pvc)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]创建PVC %s，发生错误", cluster.Namespace, cluster.Name, pvcName))
			return created, err
		} else {
			created = true
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]创建PVC %s完成", cluster.Namespace, cluster.Name, pvcName))
		}
	} else if needUpdatePVC(existPvc, request) {
		r.Log.V(1).Info(fmt.Sprintf("[%s:%s]更新PVC %s", cluster.Namespace, cluster.Name, pvcName))
		err := r.client.Update(context.TODO(), existPvc)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]更新PVC %s，发生错误", cluster.Namespace, cluster.Name, pvcName))
			return created, err
		}
	}
	return created, nil
}

/*
维护Service
方法参数：
	cluster：当前CR
	write：是否是写服务
*/
func (r *resourceService) ensureService(cluster *opengaussv1.OpenGaussCluster, write bool) error {
	existSvc, exist, err := r.findService(cluster, write)
	if err != nil {
		return err
	}

	if !exist {
		svc, err := r.newService(cluster, write)
		if err != nil {
			return err
		}
		err = r.client.Create(context.TODO(), svc)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]创建Service %s，发生错误", cluster.Namespace, cluster.Name, cluster.GetServiceName(write)))
			return err
		}
	} else if needUpdateService(cluster, existSvc, write) {
		err := r.client.Update(context.TODO(), existSvc)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]更新Service %s，发生错误", cluster.Namespace, cluster.Name, cluster.GetServiceName(write)))
			return err
		}
	}

	return nil
}

/*
维护集群默认的Secret
*/
func (r *resourceService) EnsureSecret(cluster *opengaussv1.OpenGaussCluster) error {
	exist, err := r.findSecret(cluster)
	if err != nil {
		return err
	}

	if !exist {
		sc, err := r.newSecret(cluster)
		if err != nil {
			return err
		}
		err = r.client.Create(context.TODO(), sc)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]创建Secret %s，发生错误", cluster.Namespace, cluster.Name, cluster.GetSecretName()))
			return err
		}
	}

	return nil
}

/*
检查ConfigMap，Service和Secret是否正常
*/
func (r *resourceService) CheckClusterArtifacts(cluster *opengaussv1.OpenGaussCluster) error {
	errs := make([]error, 0)

	if cmErrs := r.checkConfigMaps(cluster); len(cmErrs) > 0 {
		errs = append(errs, cmErrs...)
	}

	if svcErrs := r.checkServices(cluster); len(svcErrs) > 0 {
		errs = append(errs, svcErrs...)
	}

	if err := r.checkSecret(cluster); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		errorMessage := make([]string, 0)
		for _, e := range errs {
			errorMessage = append(errorMessage, e.Error())
		}
		return fmt.Errorf(utils.StringArrayToString(errorMessage))
	}

	return nil
}

func (r *resourceService) checkConfigMaps(cluster *opengaussv1.OpenGaussCluster) []error {
	errs := make([]error, 0)
	if e := r.checkConfigMap(cluster, cluster.GetConfigMapName()); e != nil {
		errs = append(errs, e)
	}
	if e := r.checkConfigMap(cluster, cluster.GetValidSpec().ScriptConfig); e != nil {
		errs = append(errs, e)
	}
	if e := r.checkConfigMap(cluster, cluster.GetValidSpec().FilebeatConfig); e != nil {
		errs = append(errs, e)
	}
	return errs
}

func (r *resourceService) checkConfigMap(cluster *opengaussv1.OpenGaussCluster, cmName string) error {
	if _, exist, err := r.findConfigMap(cluster.Namespace, cluster.Name, cmName); err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("ConfigMap %s不存在", cmName)
	}
	return nil
}
func (r *resourceService) checkSecret(cluster *opengaussv1.OpenGaussCluster) error {
	if exist, err := r.findSecret(cluster); err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("Secret %s不存在", cluster.GetSecretName())
	}
	return nil
}
func (r *resourceService) checkServices(cluster *opengaussv1.OpenGaussCluster) []error {
	errs := make([]error, 0)
	readSvcErrs := r.checkService(cluster, false)
	if len(readSvcErrs) > 0 {
		errs = append(errs, readSvcErrs...)
	}
	writeSvcErrs := r.checkService(cluster, true)
	if len(writeSvcErrs) > 0 {
		errs = append(errs, writeSvcErrs...)
	}
	return errs
}
func (r *resourceService) checkService(cluster *opengaussv1.OpenGaussCluster, write bool) []error {
	errs := make([]error, 0)
	svcName := cluster.GetServiceName(write)
	if svc, exist, err := r.findService(cluster, write); err != nil {
		errs = append(errs, err)
	} else if !exist {
		errs = append(errs, fmt.Errorf("Service %s不存在", svcName))
	} else {
		port := svc.Spec.Ports[0]
		svcPort := cluster.GetValidSpec().ReadPort
		expectRoleValue := OG_DB_ROLE_STANDBY
		if write {
			svcPort = cluster.GetValidSpec().WritePort
			expectRoleValue = OG_DB_ROLE_PRIMARY
		}
		dbPort := cluster.GetValidSpec().DBPort
		if port.NodePort != svcPort {
			errs = append(errs, fmt.Errorf("Service %s的服务端口%d与期望端口%d不一致", svcName, port.NodePort, svcPort))
		}
		if port.Port != dbPort {
			errs = append(errs, fmt.Errorf("Service %s的应用端口%d与期望端口%d不一致", svcName, port.Port, dbPort))
		}
		if port.TargetPort.IntValue() != int(dbPort) {
			errs = append(errs, fmt.Errorf("Service %s的目标端口%d与期望端口%d不一致", svcName, port.TargetPort.IntValue(), dbPort))
		}

		if write {

		}
		selector := svc.Spec.Selector
		expectSelector := make(map[string]string, 2)
		expectSelector[OPENGAUSS_CLUSTER_KEY] = cluster.Name
		expectSelector[OPENGAUSS_ROLE_KEY] = expectRoleValue
		if !utils.CompareMaps(selector, expectSelector) {
			errs = append(errs, fmt.Errorf("Service %s的选择器与期望不一致", svcName))
		}
	}
	return errs
}

func (r *resourceService) findConfigMap(namespace, name, cmName string) (*corev1.ConfigMap, bool, error) {
	cm, err := r.GetConfigMap(namespace, cmName)
	if err != nil {
		if errors.IsNotFound(err) {
			return cm, false, nil
		} else {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询ConfigMap %s，发生错误", namespace, name, cmName))
			return cm, false, err
		}
	}
	return cm, true, nil
}

func (r *resourceService) findService(cluster *opengaussv1.OpenGaussCluster, write bool) (*corev1.Service, bool, error) {
	svcName := cluster.GetServiceName(write)
	existSvc, err := r.getService(cluster.Namespace, svcName)
	if err != nil {
		if errors.IsNotFound(err) {
			return existSvc, false, nil
		} else {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询Service %s，发生错误", cluster.Namespace, cluster.Name, svcName))
			return existSvc, false, err
		}
	}
	return existSvc, true, nil
}

func (r *resourceService) findSecret(cluster *opengaussv1.OpenGaussCluster) (bool, error) {
	secretName := cluster.GetSecretName()
	_, err := r.getSecret(cluster.Namespace, secretName)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		} else {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询Secret %s，发生错误", cluster.Namespace, cluster.Name, secretName))
			return false, err
		}
	}
	return true, nil
}

func (r *resourceService) GetPod(namespace, name string) (*corev1.Pod, error) {
	existPod := &corev1.Pod{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, existPod)
	return existPod, err
}

func (r *resourceService) DeletePod(pod *corev1.Pod) error {
	return r.client.Delete(context.TODO(), pod)
}
func (r *resourceService) ForceDeletePod(pod *corev1.Pod) error {
	deleteOps := client.DeleteOptions{}
	var gs int64 = 0
	deleteOps.GracePeriodSeconds = &gs
	return r.client.Delete(context.TODO(), pod, &deleteOps)
}

func (r *resourceService) DeleteConfigMap(namespace, name string) error {
	cm := &corev1.ConfigMap{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, cm); err != nil {
		return err
	} else {
		return r.client.Delete(context.TODO(), cm)
	}
}

/*
检查Pod是否正常
方法参数：
	pod：当前Pod
	port：数据库端口
返回值：
	Pod是否正常
方法逻辑：
	如果Pod是running状态且ping的结果正常，返回true，否则返回false
*/
func (r *resourceService) IsPodReady(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	dbPort := getDBPort(&pod)
	if err := utils.Ping(pod.Status.PodIP, dbPort); err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s]的Pod %s telnet ip and port fail", pod.Namespace, pod.Name))
		return false
	}
	return true
}

/**
检查Pod是否正常
方法参数：
	pod：当前Pod
	port：数据库端口
返回值：
	Pod是否正常
方法逻辑：
	如果Pod是running状态但连续5次ping的结果失败，返回false，否则返回true
*/
func (r *resourceService) IsDbContainerPortReady(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	retryCount := int32(1)
	dbPort := getDBPort(&pod)
	for {
		if err := utils.Ping(pod.Status.PodIP, dbPort); err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]的Pod %s 第%d次telnet ip and port fail", getCRName(&pod), pod.Namespace, pod.Name, retryCount))
			retryCount++
		} else {
			return true
		}
		if retryCount > 5 {
			return false
		}
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}

}
func (r *resourceService) deletePVC(pvc *corev1.PersistentVolumeClaim) error {
	return r.client.Delete(context.TODO(), pvc)
}
func (r *resourceService) deletePV(pv *corev1.PersistentVolume) error {
	return r.client.Delete(context.TODO(), pv)
}

func (r *resourceService) getPV(name string) (*corev1.PersistentVolume, error) {
	existPV := &corev1.PersistentVolume{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name: name,
	}, existPV)
	return existPV, err
}

func (r *resourceService) getPVC(namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	existPVC := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, existPVC)
	return existPVC, err
}

func (r *resourceService) getService(namespace, name string) (*corev1.Service, error) {
	svc := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, svc)
	return svc, err
}

func (r *resourceService) GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, configMap)
	return configMap, err
}

func (r *resourceService) getSecret(namespace, name string) (*corev1.Secret, error) {
	sc := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, sc)
	return sc, err
}

func (r *resourceService) GetCluster(namespace, name string) (*opengaussv1.OpenGaussCluster, error) {
	cluster := &opengaussv1.OpenGaussCluster{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, cluster)
	return cluster, err
}
func (r *resourceService) FindPodsByCluster(cluster *opengaussv1.OpenGaussCluster, sort bool) ([]corev1.Pod, error) {
	searchLabels := map[string]string{
		OPENGAUSS_APP_KEY:     OPENGAUSS_APP_VAL,
		OPENGAUSS_CLUSTER_KEY: cluster.Name,
	}
	pods := &corev1.PodList{}
	err := r.client.List(context.TODO(), pods, client.InNamespace(cluster.Namespace), client.MatchingLabels(searchLabels))
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]查询集群Pod，发生错误", cluster.Namespace, cluster.Name))
		return pods.Items, err
	}
	if sort {
		return sortPods(pods, cluster.GetValidSpec().IpList), nil
	} else {
		return pods.Items, nil
	}

}

func sortPods(pods *corev1.PodList, ipList []opengaussv1.IpNodeEntry) []corev1.Pod {
	currIndex := len(ipList)
	podsToSort := make([]SortPod, 0)
	for _, pod := range pods.Items {
		podIP := pod.Status.PodIP
		if podIP == "" {
			continue
		}
		configured := false
		podIndex := 0
		for i, e := range ipList {
			if e.Ip == podIP {
				configured = true
				podIndex = i
				break
			}
		}
		if !configured {
			podIndex = currIndex
			currIndex++
		}
		sp := SortPod{
			Pod:   pod,
			Index: podIndex,
		}
		podsToSort = append(podsToSort, sp)
	}
	sort.Slice(podsToSort, func(i, j int) bool {
		return podsToSort[i].Index < podsToSort[j].Index
	})
	sortedPods := make([]corev1.Pod, 0)
	for _, sp := range podsToSort {
		sortedPods = append(sortedPods, sp.Pod)
	}
	return sortedPods
}

type SortPod struct {
	Pod   corev1.Pod
	Index int
}

func (r *resourceService) AddRoleLabelToPod(namespace, podName string, primary bool) error {
	if pod, e := r.GetPod(namespace, podName); e != nil {
		return e
	} else {
		labelRole := OG_DB_ROLE_STANDBY
		if primary {
			labelRole = OG_DB_ROLE_PRIMARY
		}
		roleVal, exist := pod.Labels[OPENGAUSS_ROLE_KEY]
		if !exist || roleVal != labelRole {
			name := getCRName(pod)
			return r.ensurePodLabel(pod, namespace, name, OPENGAUSS_ROLE_KEY, labelRole)
		}
		return nil
	}
}

func (r *resourceService) RemoveRoleLabelFromPod(namespace string, pod *corev1.Pod) error {
	if _, exist := pod.Labels[OPENGAUSS_ROLE_KEY]; exist {
		name := getCRName(pod)
		return r.ensurePodLabel(pod, namespace, name, OPENGAUSS_ROLE_KEY, "")
	}
	return nil
}

/*
维护Pod Label
方法参数：
	pod：当前Pod
	namespace：CR namespace
	name：CR name
	key： Label名称
	value： Label的期望值
方法逻辑：
	根据传入的key和value修改Pod Label，
	如果发生冲突，则重新查询Pod，再次尝试更新
	更新成功后，不断查询Pod信息，直至查到的Label与传入值一致，或超时
*/
func (r *resourceService) ensurePodLabel(pod *corev1.Pod, namespace, name, key, value string) error {
	labels := pod.GetLabels()
	if value == "" {
		delete(labels, key)
	} else {
		labels[key] = value
	}
	pod.SetLabels(labels)
	retryCount := 0
	for {
		err := r.client.Update(context.TODO(), pod)
		if err != nil && errors.IsConflict(err) {
			if errors.IsConflict(err) {
				retryCount++
				if retryCount > utils.RETRY_LIMIT {
					return err
				}
				r.Log.V(1).Info(fmt.Sprintf("[%s:%s]更新Pod %s的labe发生冲突，将于%d秒后进行第%d次重试", namespace, name, pod.Name, utils.RETRY_INTERVAL, retryCount))
				time.Sleep(time.Second * utils.RETRY_INTERVAL)
				pod, err = r.GetPod(pod.Namespace, pod.Name)
				if err != nil {
					return err
				}
			} else {
				r.Log.Error(err, fmt.Sprintf("[%s:%s]更新Pod %s的label，发生错误", namespace, name, pod.Name))
				return err
			}
		} else {
			break
		}
	}
	retryCount = 0
	for {
		pod, err := r.GetPod(pod.Namespace, pod.Name)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询Pod %s，发生错误", namespace, name, pod.Name))
			return err
		}
		v, exist := pod.Labels[key]
		if (!exist && value == "") || (exist && v == value) {
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod %s的label更新完成", namespace, name, pod.Name))
			return nil
		}
		retryCount++
		if retryCount > utils.RETRY_LIMIT {
			return fmt.Errorf("[%s:%s]更新Pod %s的label超时", namespace, name, pod.Name)
		} else {
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]更新Pod %s的label未完成，将于%d后进行第%d次重试", namespace, name, pod.Name, utils.RETRY_INTERVAL, retryCount))
			time.Sleep(time.Second * utils.RETRY_INTERVAL)
		}
	}
}

func (r *resourceService) newPod(cluster *opengaussv1.OpenGaussCluster, entry opengaussv1.IpNodeEntry, pvcNewCreate bool) (*corev1.Pod, error) {
	params := make(map[string]string, 0)
	GetParamsWithObjReference(cluster, params)
	spec := cluster.GetValidSpec()
	params[POD_NAME] = cluster.GetPodName(entry.Ip)
	params[POD_IP] = entry.Ip
	params[POD_DB_IMG] = spec.Image
	params[DB_CPU_LMT] = spec.Cpu
	params[DB_CPU_REQ] = opengaussv1.DB_CPU_REQ
	params[DB_MEM_LMT] = spec.Memory
	params[DB_MEM_REQ] = opengaussv1.DB_MEM_REQ
	if spec.SidecarImage != "" {
		params[POD_SIDECAR_IMG] = spec.SidecarImage
	} else {
		params[POD_SIDECAR_IMG] = spec.Image
	}
	params[SIDECAR_CPU_LMT] = spec.SidecarCpu
	params[SIDECAR_CPU_REQ] = opengaussv1.SIDECAR_CPU_REQ
	params[SIDECAR_MEM_LMT] = spec.SidecarMemory
	params[SIDECAR_MEM_REQ] = opengaussv1.SIDECAR_MEM_REQ
	params[CR_SECRET_NAME] = cluster.GetSecretName()
	params[POD_NODE_SELECT] = entry.NodeName
	params[CR_BACKUP_PATH] = spec.BackupPath
	params[CR_ARCHIVE_PATH] = spec.ArchiveLogPath
	params[CR_DB_PORT] = fmt.Sprint(spec.DBPort)
	params[CLUSTER_CM_NAME] = OG_CLUSTER_CONFIGMAP_NAME
	params[CLUSTER_CM_VAL] = cluster.GetConfigMapName()
	params[SCRIPT_CM_NAME] = OG_SCRIPT_CONFIGMAP_NAME
	params[SCRIPT_CM_VAL] = spec.ScriptConfig
	params[FILEBEAT_CM_NAME] = OG_FILEBEAT_CONFIGMAP_NAME
	params[FILEBEAT_CM_VAL] = spec.FilebeatConfig
	params[GRACE_PERIOD] = fmt.Sprint(spec.Schedule.GracePeriod)
	params[TOLERATION_SECOND] = fmt.Sprint(spec.Schedule.Toleration)
	//根据cr配置中的探针周期时间设置pod的探活周期
	params[LIVENESS_PROBE_PERIOD] = fmt.Sprint(spec.Schedule.LivenessProbePeriod)
	params[READINESS_PROBE_PERIOD] = fmt.Sprint(spec.Schedule.ReadinessProbePeriod)
	if pvcNewCreate {
		params[NEED_INIT] = "true"
		r.Log.V(1).Info(fmt.Sprintf("[%s:%s:%s] pvc is new create ,need init", cluster.Namespace, cluster.Name, params[POD_NAME]))
	} else {
		params[NEED_INIT] = "false"
		r.Log.V(1).Info(fmt.Sprintf("[%s:%s:%s]pvc is already exists ,not need init", cluster.Namespace, cluster.Name, params[POD_NAME]))
	}
	yamlPod := GetResourceYaml(YAML_POD, params)

	pod := &corev1.Pod{}
	err := yaml.Unmarshal([]byte(yamlPod), pod)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]将文本转换为Pod，发生错误， 文本内容：%s", cluster.Namespace, cluster.Name, yamlPod))
		return pod, err
	}
	r.setPodBandwidth(pod, cluster)
	r.SetContainerCustomizedenv(cluster, pod)
	//更新pod的label
	r.updatePodLabel(pod, cluster.GetValidSpec().Labels, cluster.Status.Spec.Labels, true)
	//更新pod的annotation
	r.updatePodAnnotations(pod, cluster, entry, true)
	//设置pod的节点亲和性，网络插件为calico，则node亲和性值为entry.Nodename
	if entry.NodeName != "" && opengaussv1.NETWORK_CALICO == spec.NetworkClass {
		var podNodeSelect []string
		podNodeSelect = append(podNodeSelect, entry.NodeName)
		r.updatePodAffinity(cluster, podNodeSelect, pod)
	} else {
		r.updatePodAffinity(cluster, spec.Schedule.Nodes, pod)
	}

	return pod, nil
}

func (r *resourceService) newPVC(cluster *opengaussv1.OpenGaussCluster, ip, pvcType, request string) (*corev1.PersistentVolumeClaim, error) {
	params := make(map[string]string)
	GetParamsWithObjReference(cluster, params)
	params[CR_NAMESPACE] = cluster.Namespace
	params[POD_NAME] = cluster.GetPodName(ip)
	params[PVC_TYPE] = pvcType
	params[PVC_STORAGE_REQ] = request
	params[PVC_STORAGE_CLASS] = cluster.GetValidSpec().StorageClass

	sourceYaml := YAML_PVC
	if cluster.IsHostpathEnable() {
		sourceYaml = YAML_PVC_HOSTPATH
	}
	yamlPvc := GetResourceYaml(sourceYaml, params)

	pvc := &corev1.PersistentVolumeClaim{}
	err := yaml.Unmarshal([]byte(yamlPvc), pvc)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]将文本转换为%s类型的PVC，发生错误，文本内容：%s", cluster.Namespace, cluster.Name, pvcType, yamlPvc))
	}
	return pvc, err
}

func (r *resourceService) newPV(cluster *opengaussv1.OpenGaussCluster, entry opengaussv1.IpNodeEntry, pvType, limit string) (*corev1.PersistentVolume, error) {
	params := make(map[string]string)
	GetParamsWithObjReference(cluster, params)
	params[POD_NAME] = cluster.GetPodName(entry.Ip)
	params[PV_TYPE] = pvType
	params[PV_STORAGE_CAPACITY] = limit
	params[PV_NODE_SELECT] = entry.NodeName
	params[HOSTPATH_ROOT] = cluster.GetValidSpec().HostpathRoot
	yamlPv := GetResourceYaml(YAML_PV, params)

	pv := &corev1.PersistentVolume{}
	err := yaml.Unmarshal([]byte(yamlPv), pv)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]将文本转换为%s类型的PV，发生错误，文本内容：%s", cluster.Namespace, cluster.Name, pvType, yamlPv))
	}
	return pv, err
}

func (r *resourceService) newService(cluster *opengaussv1.OpenGaussCluster, write bool) (*corev1.Service, error) {
	params := make(map[string]string, 0)
	GetParamsWithObjReference(cluster, params)
	serviceName := cluster.GetServiceName(write)
	params[SVC_NAME] = serviceName
	port := cluster.GetValidSpec().ReadPort
	role := OG_DB_ROLE_STANDBY
	if write {
		port = cluster.GetValidSpec().WritePort
		role = OG_DB_ROLE_PRIMARY
	}
	params[CR_SVC_PORT] = fmt.Sprint(port)
	params[DB_ROLE] = role
	params[CR_DB_PORT] = fmt.Sprint(cluster.GetValidSpec().DBPort)

	yamlSvc := GetResourceYaml(YAML_SVC, params)

	svc := &corev1.Service{}
	err := yaml.Unmarshal([]byte(yamlSvc), svc)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]将文本转换为Service %s，发生错误，文本内容：%s", cluster.Namespace, cluster.Name, serviceName, yamlSvc))
	}
	return svc, err
}

func (r *resourceService) newClusterConfigMap(cluster *opengaussv1.OpenGaussCluster) (*corev1.ConfigMap, error) {
	params := make(map[string]string, 0)
	GetParamsWithObjReference(cluster, params)
	params[CLUSTER_CM_NAME] = cluster.GetConfigMapName()
	params[CR_DB_PORT] = fmt.Sprint(cluster.GetValidSpec().DBPort)
	yamlMap := GetResourceYaml(YAML_CM, params)

	cm := &corev1.ConfigMap{}
	err := yaml.Unmarshal([]byte(yamlMap), cm)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]将文本转换为ConfigMap，发生错误，文本内容：%s", cluster.Namespace, cluster.Name, yamlMap))
	}
	return cm, err
}

func (r *resourceService) newScriptConfigMap(cluster *opengaussv1.OpenGaussCluster) (*corev1.ConfigMap, error) {
	params := make(map[string]string, 0)
	params[CR_NAMESPACE] = cluster.Namespace
	params[SCRIPT_CM_NAME] = cluster.GetValidSpec().ScriptConfig
	yamlMap := GetResourceYaml(YAML_SCRIPT_CM, params)

	cm := &corev1.ConfigMap{}
	err := yaml.Unmarshal([]byte(yamlMap), cm)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]将文本转换为ConfigMap，发生错误，文本内容：%s", cluster.Namespace, cluster.Name, yamlMap))
	}
	return cm, err
}

/**
filebeat支持通过设置operator的环境变量设置
*/
func (r *resourceService) newFilebeatConfigMap(namespace, cmName string) (*corev1.ConfigMap, error) {
	params := make(map[string]string, 0)
	params[FILEBEAT_CM_NAME] = cmName
	params[CR_NAMESPACE] = namespace
	yamlMap := GetResourceYaml(YAML_FILEBEAT_CM, params)
	cm := &corev1.ConfigMap{}
	err := yaml.Unmarshal([]byte(yamlMap), cm)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]将文本转换为ConfigMap，发生错误，文本内容：%s", namespace, cmName, yamlMap))
	}
	return cm, err
}

func (r *resourceService) newSecret(cluster *opengaussv1.OpenGaussCluster) (*corev1.Secret, error) {
	params := make(map[string]string, 0)
	GetParamsWithObjReference(cluster, params)

	yamlSc := GetResourceYaml(YAML_SECRET, params)

	sc := &corev1.Secret{}
	err := yaml.Unmarshal([]byte(yamlSc), sc)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]将文本转换为Secret，发生错误，文本内容：%s", cluster.Namespace, cluster.Name, yamlSc))
	}
	return sc, err
}

func needUpdatePV(pv *corev1.PersistentVolume, limit string) bool {
	resources := pv.Spec.Capacity
	if !utils.CompareResource(resources, corev1.ResourceStorage, limit) {
		quantity, e := resource.ParseQuantity(limit)
		if e != nil {
			return false
		}
		resources[corev1.ResourceStorage] = quantity
		return true
	} else {
		return false
	}
}

func needUpdatePVC(pvc *corev1.PersistentVolumeClaim, request string) bool {
	update := false
	resources := pvc.Spec.Resources.Requests
	if !utils.CompareResource(resources, corev1.ResourceStorage, request) {
		quantity, e := resource.ParseQuantity(request)
		if e != nil {
			return false
		}
		resources[corev1.ResourceStorage] = quantity
		update = true
	}
	return update
}

func needUpdateService(cluster *opengaussv1.OpenGaussCluster, svc *corev1.Service, write bool) bool {
	dbPort := cluster.GetValidSpec().DBPort
	svcPort := cluster.GetValidSpec().ReadPort
	expectRoleValue := OG_DB_ROLE_STANDBY
	if write {
		svcPort = cluster.GetValidSpec().WritePort
		expectRoleValue = OG_DB_ROLE_PRIMARY
	}
	update := false
	if svcPort != svc.Spec.Ports[0].NodePort {
		svc.Spec.Ports[0].NodePort = svcPort
		update = true
	}
	if dbPort != svc.Spec.Ports[0].Port {
		svc.Spec.Ports[0].Port = dbPort
		update = true
	}
	if int(dbPort) != svc.Spec.Ports[0].TargetPort.IntValue() {
		svc.Spec.Ports[0].TargetPort = intstr.FromInt(int(dbPort))
		update = true
	}
	selector := svc.Spec.Selector
	expectSelector := make(map[string]string, 2)
	expectSelector[OPENGAUSS_CLUSTER_KEY] = cluster.Name
	expectSelector[OPENGAUSS_ROLE_KEY] = expectRoleValue
	if !utils.CompareMaps(selector, expectSelector) {
		svc.Spec.Selector = expectSelector
		update = true
	}
	return update
}

func (r *resourceService) IsPodMatchWithSpec(cluster *opengaussv1.OpenGaussCluster, pod corev1.Pod) bool {
	if cluster.Status.Spec.Schedule.MostAvailableTimeout == 0 {
		return false
	}
	ogContainer := pod.Spec.Containers[0]
	if !isContainerMatchWithSpec(ogContainer, cluster.Spec.Image, cluster.Spec.Cpu, cluster.Spec.Memory) {
		return false
	}
	//比较 og容器的readinessProbe的Period是否发生变化
	if cluster.Spec.Schedule.ReadinessProbePeriod != ogContainer.ReadinessProbe.PeriodSeconds {
		return false
	}
	if !r.isContainerEnvMatchWithSpec(cluster, pod) {
		r.Log.Info(fmt.Sprintf("[%s:%s]的pod[%s]自定义环境变量发生变化，需要升级", cluster.Namespace, cluster.Name, pod.Name))
		return false
	}

	sidecarContainer := pod.Spec.Containers[1]
	sidecarImage := cluster.Spec.SidecarImage
	if sidecarImage == "" {
		sidecarImage = cluster.Spec.Image
	}
	if !isContainerMatchWithSpec(sidecarContainer, sidecarImage, cluster.Spec.SidecarCpu, cluster.Spec.SidecarMemory) {
		return false
	}

	if !isBandWidthMatch(cluster.Spec.BandWidth, cluster.Spec.NetworkClass, pod.GetAnnotations()) {
		return false
	}
	if !r.IsExtendIpMatch(cluster, pod, cluster.GetIpExtendIpMap()) {
		return false
	}
	volumes := pod.Spec.Volumes
	clusterCfgMatch := false
	scriptCfgMatch := false
	filebeatCfgMatch := false
	for _, v := range volumes {
		if v.Name == OG_CLUSTER_CONFIGMAP_NAME {
			clusterCfgMatch = v.ConfigMap.Name == cluster.GetConfigMapName()
		}
		if v.Name == OG_SCRIPT_CONFIGMAP_NAME {
			scriptCfgMatch = v.ConfigMap.Name == cluster.GetValidSpec().ScriptConfig
		}
		if v.Name == OG_FILEBEAT_CONFIGMAP_NAME {
			filebeatCfgMatch = v.ConfigMap.Name == cluster.GetValidSpec().FilebeatConfig
		}
	}
	return clusterCfgMatch && scriptCfgMatch && filebeatCfgMatch
}

func isBandWidthMatch(specVal, networkClass string, annotation map[string]string) bool {
	if opengaussv1.NETWORK_KUBE_OVN != networkClass {
		annoVal, exist := annotation[BANDWIDTH_INGRESS_KEY]
		if !exist && specVal == "" {
			return true
		} else if exist && specVal != "" {
			fixedVal := utils.GetFixedBandWidth(specVal)
			return annoVal == fixedVal
		} else {
			return false
		}
	} else {
		return true
	}
}

func isContainerMatchWithSpec(container corev1.Container, image, cpu, mem string) bool {
	if container.Image != image {
		return false
	}
	resource := container.Resources.Limits
	if !utils.CompareResource(resource, corev1.ResourceCPU, cpu) || !utils.CompareResource(resource, corev1.ResourceMemory, mem) {
		return false
	}
	return true
}

/*
等待集群的所有与配置IP无关的Pod删除完成
*/
func (r *resourceService) WaitPodsCleanup(cluster *opengaussv1.OpenGaussCluster) bool {
	ipSet := cluster.Spec.IpSet()
	retryCount := int32(0)
	timeout := cluster.GetValidSpec().Schedule.ProcessTimeout
	for {
		pods, err := r.FindPodsByCluster(cluster, false)
		if err == nil {
			complete := true
			for _, pod := range pods {
				r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod %s处于%s阶段", cluster.Namespace, cluster.Name, pod.Name, string(pod.Status.Phase)))
				//如果有一个Pod的IP不在IpList中，就标记为未完成，等待重试
				if !ipSet.Contains(pod.Status.PodIP) {
					complete = false
					break
				}
			}
			if complete {
				r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod清理完成", cluster.Namespace, cluster.Name))
				return true
			}
		} else {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询Pod，发生错误", cluster.Namespace, cluster.Name))
		}
		if retryCount*utils.RETRY_INTERVAL >= timeout {
			r.Log.Info(fmt.Sprintf("[%s:%s]清理Pod超时", cluster.Namespace, cluster.Name))
			return false
		} else {
			retryCount++
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]清理Pod未完成，将于%d秒后进行第%d次重试", cluster.Namespace, cluster.Name, utils.RETRY_INTERVAL, retryCount))
		}
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
}

/*
等待集群的所有Pod进入running状态
*/
func (r *resourceService) WaitPodsRunning(cluster *opengaussv1.OpenGaussCluster) ([]corev1.Pod, bool) {
	// wait for all pods running
	retryCount := int32(0)
	timeout := cluster.GetValidSpec().Schedule.ProcessTimeout
	expectIpSet := cluster.GetValidSpec().IpSet()
	for {
		//查询当前可以查到status.pod.podip的ipset
		pods, err := r.FindPodsByCluster(cluster, true)
		if err == nil {
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod个数:%v", cluster.Namespace, cluster.Name, len(pods)))
			if len(pods) > 0 {
				ready := true
				actualSet := utils.NewSet()
				for _, pod := range pods {
					podIP := pod.Status.PodIP
					r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod %s处于%s阶段,所在node:%s", cluster.Namespace, cluster.Name, pod.Name, string(pod.Status.Phase), pod.Spec.NodeName))
					//如果有一个Pod状态不正常，就结束本轮，等待重试
					if !r.IsPodReady(pod) {
						ready = false
						break
					}
					actualSet.Add(podIP)
				}
				if ready && actualSet.ContainsAll(cluster.GetValidSpec().IpSet()) {
					r.Log.V(1).Info(fmt.Sprintf("[%s:%s]所有的Pod都处于运行状态", cluster.Namespace, cluster.Name))
					return pods, true
				}

				if ready {
					//判断当前集群期望的pod是否包含因failmount/failedattachvolume处于init状态，且持续时间超过5分钟，
					// 若存在，且超过5分钟，，直接跳出循环，以已经running的pod组建集群
					//FindPodsByCluster(cluster, false)查询， sort参数传false，否则查不到当前init状态的pod
					actPods, _ := r.FindPodsByCluster(cluster, false)
					expectIpSet := cluster.GetValidSpec().IpSet()
					//beyondIpSet, _ := expectIpSet.DiffTo(actualSet)
					inSpecPodNum := 0
					for key, _ := range expectIpSet.Map {
						if actualSet.Contains(key) {
							inSpecPodNum++
						}
					}
					r.Log.V(1).Info(fmt.Sprintf("[%s:%s]期望的Pods：%v,实际running的pods: %v,当前期望的pods运行个数为:%d", cluster.Namespace, cluster.Name, expectIpSet, actualSet, inSpecPodNum))
					noRunningIpSet := utils.NewSet()
					//过滤当前没有running的（init状态的），且失败原因是因为failmount/failedattachvolume的pod
					for _, unNormalPod := range actPods {
						unnormalPodIp := cluster.GetPodIpByPodName(unNormalPod.Name)
						for podIp, _ := range actualSet.Map {
							if podIp != unnormalPodIp && expectIpSet.Contains(unnormalPodIp) {
								failmount, _ := r.IsIncludeFailMountEvents(cluster.Namespace, cluster.Name, unNormalPod.Name)
								if failmount && unNormalPod.Status.Phase == corev1.PodPending && time.Now().Sub(unNormalPod.CreationTimestamp.Time).Seconds() > 300 {
									r.Log.Info(fmt.Sprintf("[%s:%s]的pod %s处于%s状态,持续时间为%f s", cluster.Namespace, cluster.Name, unNormalPod.Name, unNormalPod.Status.Phase,
										time.Now().Sub(unNormalPod.CreationTimestamp.Time).Seconds()))
									inSpecPodNum = inSpecPodNum + 1
									noRunningIpSet.Add(unnormalPodIp)
								}
							}
						}
					}
					//判断当前cr spec.ipset中是否为以下两种状态 ：running or 因failmount/failedattachvolume而一致init状态
					if inSpecPodNum == cluster.GetValidSpec().IpSet().Size() {
						r.Log.V(1).Info(fmt.Sprintf("[%s:%s]期望的pods[%v]中有如下Pods:[%v]处于init状态", cluster.Namespace, cluster.Name, expectIpSet.String(), noRunningIpSet.String()))
						return pods, true
					}
				}

			} else {
				//判断当前集群是否所有pod因failmount/failedattachvolume处于init状态，且持续时间超过5分钟，超过5分钟，则本次不等待5分钟，直接跳出循环
				allPodFailMount := true
				//否则查不到当前init状态的pod
				actPods, _ := r.FindPodsByCluster(cluster, false)
				noRunningIpSet := utils.NewSet()
				//过滤当前没有running的（init状态的），且失败原因是因为failmount/failedattachvolume的pod
				for _, unNormalPod := range actPods {
					if retryCount*utils.RETRY_INTERVAL >= timeout {
						r.Log.Info(fmt.Sprintf("[%s:%s]Pod %s持续%d秒未启动成功，超时", cluster.Namespace, cluster.Name, unNormalPod.Name, timeout))
						r.Log.V(1).Info(r.GetPodEvents(cluster.Namespace, cluster.Name, unNormalPod.Name))
					}
					unnormalPodIp := cluster.GetPodIpByPodName(unNormalPod.Name)
					if expectIpSet.Contains(unnormalPodIp) {
						failmount, _ := r.IsIncludeFailMountEvents(cluster.Namespace, cluster.Name, unNormalPod.Name)
						if failmount && time.Now().Sub(unNormalPod.CreationTimestamp.Time).Seconds() > 300 {
							r.Log.Info(fmt.Sprintf("[%s:%s]的pod %s处于%s状态,持续时间为%f s", cluster.Namespace, cluster.Name, unNormalPod.Name, unNormalPod.Status.Phase,
								time.Now().Sub(unNormalPod.CreationTimestamp.Time).Seconds()))
							noRunningIpSet.Add(unnormalPodIp)
						} else {
							allPodFailMount = false
							break
						}
					}
				}
				r.Log.V(1).Info(fmt.Sprintf("[%s:%s]实际的pod个数为%d", cluster.Namespace, cluster.Name, len(actPods)))
				if len(actPods) > 0 && allPodFailMount {
					r.Log.V(1).Info(fmt.Sprintf("[%s:%s]期望的pods[%v]中有如下Pods:[%v]处于init状态", cluster.Namespace, cluster.Name, expectIpSet.String(), noRunningIpSet.String()))
					return pods, false
				}

			}
		} else {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询Pod，发生错误", cluster.Namespace, cluster.Name))
		}
		if retryCount*utils.RETRY_INTERVAL >= timeout {
			r.Log.Info(fmt.Sprintf("[%s:%s]启动Pod持续%d秒，超时", cluster.Namespace, cluster.Name, timeout))
			readyPods := make([]corev1.Pod, 0)
			for _, pod := range pods {
				if r.IsPodReady(pod) {
					readyPods = append(readyPods, pod)
				} else {
					r.Log.Info(fmt.Sprintf("[%s:%s]Pod %s持续%d秒未启动成功，超时", cluster.Namespace, cluster.Name, pod.Name, timeout))
					//todo 需考虑此逻辑是否可以删除，pod如果置维护，但pod的event中包含failmount事件，
					//但是pod无法启动为pending状态，为保证running可以以running状态存在，此处的逻辑需要删除
					r.Log.Info(r.GetPodEvents(cluster.Namespace, cluster.Name, pod.Name))
					failmount, _ := r.IsIncludeFailMountEvents(cluster.Namespace, cluster.Name, pod.Name)
					if failmount && pod.Status.Phase == corev1.PodRunning && time.Now().Sub(pod.CreationTimestamp.Time).Seconds() > 300 {
						r.Log.Info(fmt.Sprintf("[%s:%s]的Pod %s处于%s状态,启动Pod持续%d秒", cluster.Namespace, cluster.Name, pod.Name, pod.Status.Phase,
							timeout))
						//此处仅删除pod出现过faimount，但是挂载成功后，300秒仍未正常拉起的pod
						r.CleanPodResource(cluster, pod.Status.PodIP)
					}
				}
			}
			return readyPods, false
		} else {
			retryCount++
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod启动未完成，将于%d秒后进行第%d次重试", cluster.Namespace, cluster.Name, utils.RETRY_INTERVAL, retryCount))
		}
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
}

/*
等待单个Pod进入running状态
*/
func (r *resourceService) WaitPodRunning(cluster *opengaussv1.OpenGaussCluster, podName string) (corev1.Pod, bool) {
	retryCount := int32(0)
	timeout := cluster.GetValidSpec().Schedule.ProcessTimeout
	for {
		pod, err := r.GetPod(cluster.Namespace, podName)
		if err == nil {
			if r.IsPodReady(*pod) {
				return *pod, true
			}
		} else {
			r.Log.Error(err, fmt.Sprintf("[%s:%s]查询Pod %s，发生错误, try again", cluster.Namespace, cluster.Name, podName))
		}
		if retryCount*utils.RETRY_INTERVAL >= timeout {
			r.Log.Info(fmt.Sprintf("[%s:%s]启动Pod %s持续%d秒，超时", cluster.Namespace, cluster.Name, podName, timeout))
			return *pod, false
		} else {
			retryCount++
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod %s启动未完成，将于%d秒后进行第%d次重试", cluster.Namespace, cluster.Name, podName, utils.RETRY_INTERVAL, retryCount))
		}
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
}

/*
等待单个Pod删除完成
*/
func (r *resourceService) WaitPodCleanup(cluster *opengaussv1.OpenGaussCluster, podName string) bool {
	// wait for all pods running
	retryCount := int32(0)
	timeout := cluster.GetValidSpec().Schedule.ProcessTimeout
	for {
		_, err := r.GetPod(cluster.Namespace, podName)
		if err != nil {
			if errors.IsNotFound(err) {
				return true
			} else {
				r.Log.Error(err, fmt.Sprintf("[%s:%s]查询Pod %s，发生错误", cluster.Namespace, cluster.Name, podName))
			}
		}
		if retryCount*utils.RETRY_INTERVAL >= timeout {
			r.Log.Info(fmt.Sprintf("[%s:%s]删除Pod %s持续%d秒，超时", cluster.Namespace, cluster.Name, podName, timeout))
			return false
		} else {
			retryCount++
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]删除Pod %s未完成，将于%d秒后进行第%d次重试", cluster.Namespace, cluster.Name, podName, utils.RETRY_INTERVAL, retryCount))
		}
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
}

/*
等待PVC删除完成
*/
func (r *resourceService) WaitPVCCleanup(cluster *opengaussv1.OpenGaussCluster, pvcName string) bool {
	// wait for all pods running
	retryCount := int32(0)
	timeout := cluster.GetValidSpec().Schedule.ProcessTimeout
	for {
		_, err := r.getPVC(cluster.Namespace, pvcName)
		if err != nil {
			if errors.IsNotFound(err) {
				return true
			} else {
				r.Log.Error(err, fmt.Sprintf("[%s:%s]查询PVC %s，发生错误", cluster.Namespace, cluster.Name, pvcName))
			}
		}
		if retryCount*utils.RETRY_INTERVAL >= timeout {
			r.Log.Info(fmt.Sprintf("[%s:%s]删除PVC %s持续%d秒，超时", cluster.Namespace, cluster.Name, pvcName, timeout))
			return false
		} else {
			retryCount++
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]删除PVC %s未完成，将于%d秒后进行第%d次重试", cluster.Namespace, cluster.Name, pvcName, utils.RETRY_INTERVAL, retryCount))
		}
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
}

/*
等待PV删除完成
*/
func (r *resourceService) WaitPVleanup(cluster *opengaussv1.OpenGaussCluster, pvName string) bool {
	// wait for all pods running
	retryCount := int32(0)
	timeout := cluster.GetValidSpec().Schedule.ProcessTimeout
	for {
		_, err := r.getPV(pvName)
		if err != nil {
			if errors.IsNotFound(err) {
				return true
			} else {
				r.Log.Error(err, fmt.Sprintf("[%s:%s]查询PV %s，发生错误", cluster.Namespace, cluster.Name, pvName))
			}
		}
		if retryCount*utils.RETRY_INTERVAL >= timeout {
			r.Log.Info(fmt.Sprintf("[%s:%s]删除PV %s持续%d秒，超时", cluster.Namespace, cluster.Name, pvName, timeout))
			return false
		} else {
			retryCount++
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]删除PV %s未完成，将于%d秒后进行第%d次重试", cluster.Namespace, cluster.Name, pvName, utils.RETRY_INTERVAL, retryCount))
		}
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
}

/*
设置Pod下的container的Env
根据定义环境变量的名称前缀，给对应pod的container设置环境变量
og前缀，为og容器设置
sidecar前缀的，为sidecar设置
*/
func (r *resourceService) SetContainerEnvs(cluster *opengaussv1.OpenGaussCluster, pod *corev1.Pod) {
	CustomizedEnvs := cluster.GetValidSpec().CustomizedEnv
	containers := pod.Spec.Containers
	for key, val := range CustomizedEnvs {
		for _, container := range containers {
			if strings.HasSuffix(key, container.Name) {
				// var corev1.EnvVar monitorType
				// var monitorType corev1.EnvVar
				// monitorType.Name = "MONITOR_TYPE"
				// monitorType.Value = "OPENGAUSS"
				// container.Env = append(container.Env, monitorType)
				custEnvName := key[len(container.Name+"_"):]
				container.Env = append(container.Env, corev1.EnvVar{Name: custEnvName, Value: val, ValueFrom: nil})
			}
		}
	}

}

/*
判断container容器是否符合Spec的自定义env期望
CustomizedEnv：自定义环境变量map，
规则如下：key为container name的前缀加环境变量名称，给对应container添加/对应的env 赋值
例如：OG_MONITOR_TYPE:OpenGauss  则给pod的og container添加环境变量 MONITOR_TYP，值为OpenGauss
*/
func (r *resourceService) SetContainerCustomizedenv(cluster *opengaussv1.OpenGaussCluster, pod *corev1.Pod) {
	customizeEnv := cluster.GetValidSpec().CustomizedEnv
	//【删除环境变量 判断status中的CustomizedEnv存在，spec中CustomizedEnv不存在的，需要从pod中删除】
	//删除环境变量的逻辑不需要考虑

	if len(customizeEnv) > 0 {
		containers := pod.Spec.Containers
		for containerPosi, container := range containers { //遍历container
			containerEnvVar := container.Env
			for ckey, cval := range customizeEnv { //遍历自定义环境变量
				if strings.HasPrefix(ckey, container.Name) {
					envExist := false //container中是否包含此环境变量
					for envPosi, envVal := range containerEnvVar {
						if strings.Contains(ckey, envVal.Name) {
							envExist = true
							//此部分代码永远不会执行，因为pod是根据yaml.go中的模板生成的，可以考虑去掉
							if cval != envVal.Value {
								r.Log.Info(fmt.Sprintf("[%s:%s] 存量集群的pod：%s，更新环境变量[%s]:old value: %s,new value: %s", cluster.Namespace, cluster.Name, pod.Name, envVal.Name, envVal.Value, cval))
								envVal.Value = cval
								containerEnvVar[envPosi] = envVal
								break
							}
						}
					}
					if !envExist {
						custEnvName := ckey[len(container.Name+"_"):]
						r.Log.Info(fmt.Sprintf("[%s:%s] 集群的pod[%s]新增自定义环境变量[%s],value: %s", cluster.Namespace, cluster.Name, pod.Name, custEnvName, cval))
						containerEnvVar = append(containerEnvVar, corev1.EnvVar{Name: custEnvName, Value: cval, ValueFrom: nil})
					}
					container.Env = containerEnvVar
				}
			}
			containers[containerPosi] = container
		}

	}
	// return nil
}

/*
判断container容器是否符合Spec的自定义env期望
CustomizedEnv: 自定义环境变量map
规则如下： key为container name的前缀加环境变量名称，给对应的container添加env
例如： og_MONITOR_TYPE: OPENGAUSS ,则给pod的og container添加环境变量 MONITOR_TYPE,值为OPENGAUSS
*/
func (r *resourceService) isContainerEnvMatchWithSpec(cluster *opengaussv1.OpenGaussCluster, pod corev1.Pod) bool {
	//自定义环境变量值是否符合期望的值
	customizeEnvMatch := true
	customizeEnv := cluster.GetValidSpec().CustomizedEnv
	if len(customizeEnv) > 0 {
		for _, container := range pod.Spec.Containers {
			containerEnvVar := container.Env
			for ckey, cval := range customizeEnv {
				if strings.HasPrefix(ckey, container.Name) {
					envExist := false //container中是否包含此环境变量
					for _, envVal := range containerEnvVar {
						if strings.Contains(ckey, envVal.Name) {
							if cval != envVal.Value {
								return false
							}
							envExist = true
						}

					}
					if !envExist {
						return false
					}
				}

			}
		}

	}
	//没有自定义环境变量，有可能是删除了所有的自定义环境变量，需要判断pod中是否包含除内置环境变量外的其他环境变量
	for _, container := range pod.Spec.Containers {
		containerEnvVar := container.Env
		for _, envVal := range containerEnvVar {
			if envVal.Name != OG_OG_PASSWORD && envVal.Name != OG_MY_POD_IP && envVal.Name != SIDECAR_CR_NAME && envVal.Name != SIDECAR_MY_POD_IP {
				if _, ok := customizeEnv[container.Name+"_"+envVal.Name]; !ok {
					r.Log.Info(fmt.Sprintf("[%s:%s] 的pod：%s的container[%s]包含spec中不存在的自定义环境变量[%s],需要升级重建", cluster.Namespace, cluster.Name, pod.Name, container.Name, envVal.Name))
					return false
				}
			}
		}
	}
	return customizeEnvMatch
}

/* 更新pod的labels
比较cr当前spec中的label与status中的label，如果发生变化，且当前pod已存在则更新pod
*/
func (r *resourceService) updatePodLabel(pod *corev1.Pod, labels, preLabels map[string]string, podCreate bool) error {
	podLabels := pod.GetLabels()
	if !utils.CompareMaps(labels, preLabels) {
		if len(preLabels) > 0 {
			for k, _ := range preLabels {
				delete(podLabels, k)
			}
		}
	}
	if len(labels) > 0 {
		for k, v := range labels {
			podLabels[k] = v
		}
	}
	pod.SetLabels(podLabels)
	if !podCreate {
		return r.client.Update(context.TODO(), pod)
	}
	return nil
}

/* 更新Pod的annotations

 */
func (r *resourceService) updatePodAnnotations(pod *corev1.Pod, cluster *opengaussv1.OpenGaussCluster, entry opengaussv1.IpNodeEntry, podCreate bool) error {
	annotations := cluster.Spec.Annotations
	preAnnotations := cluster.Status.Spec.Annotations
	networkClass := cluster.Spec.NetworkClass
	podAnnotation := pod.GetAnnotations()
	if podAnnotation == nil {
		podAnnotation = make(map[string]string)
	}
	if !utils.CompareMaps(annotations, preAnnotations) {
		if len(preAnnotations) > 0 {
			for k, _ := range preAnnotations {
				delete(podAnnotation, k)
			}
		}
	}
	if len(annotations) > 0 {
		for k, v := range annotations {
			podAnnotation[k] = v
		}
	}

	//r.Log.Info(fmt.Sprintf("[%s:%s]的pod: %s的处理前Annotation is [%v]",cluster.Namespace, cluster.Name, pod.Name, podAnnotation))
	if opengaussv1.NETWORK_CALICO == networkClass {
		podAnnotation[utils.CALICO_IP_ADDRESS_ANNOTATION] = strings.ReplaceAll(utils.CALICO_IP_ADDRESS_ANNOTATION_VAL, "POD_IP", entry.Ip)
		//业务网ip相关注解，只能在pod重建时，可以直接添加，对于已有pod，如果需要添加副网卡相关注解，需要触发升级操作，升级pod重建
	} else if opengaussv1.NETWORK_KUBE_OVN == networkClass && podCreate {
		podAnnotation[utils.KUBE_OVN_IP_ADDRESS_ANNOTATION] = entry.Ip
		if entry.ExtendIp != "" {
			attachmentNetworkVal, ok := podAnnotation[utils.ATTACHMENT_NETWORK_ANNOTATION]
			if ok {
				//获取自定义网络接口的名称和所在命名空间，即获取注解并解析，添加自定义网卡的IP注解
				if attachmentNetworkArr, err := utils.GetAttachNetworkArr(attachmentNetworkVal); err == nil {
					attachNetworkName := attachmentNetworkArr[0].Name
					attachNetworkNamespace := attachmentNetworkArr[0].Namespace
					providerName := fmt.Sprintf("%s.%s.ovn", attachNetworkName, attachNetworkNamespace)
					attachNetworkIpAddressKey := fmt.Sprintf(utils.IP_ADDRESS_ANNOTATION_TEMPLATE, providerName)
					//指定附加网卡的子网名称，并移除Pod中的attach_logical_switch_array注解
					podAnnotation[attachNetworkIpAddressKey] = entry.ExtendIp
					if attachNetworkSubnetNameArray, err := utils.GetAttachNetworkLogicSwitchArr(podAnnotation[utils.ATTACH_NETWORK_LOGICAL_SWITCH_ANNOTATION]); err != nil {
						return err
					} else {
						attachNetworkSubnetNameKey := fmt.Sprintf(utils.LOGICAL_SWITCH_ANNOTATION_TEMPLATE, providerName)
						for _, networkSubnet := range attachNetworkSubnetNameArray {
							if networkSubnet.NetworkName == attachNetworkName {
								podAnnotation[attachNetworkSubnetNameKey] = networkSubnet.SubnetName
							}
						}
					}
				} else {
					return err
				}
			}
		} else {
			delete(podAnnotation, utils.ATTACHMENT_NETWORK_ANNOTATION)
		}
		delete(podAnnotation, utils.ATTACH_NETWORK_LOGICAL_SWITCH_ANNOTATION)
	}
	//r.Log.Info(fmt.Sprintf("[%s:%s]的pod: %s根据网络插件类型设置的Annotation is [%v]",cluster.Namespace, cluster.Name, pod.Name, podAnnotation))
	pod.SetAnnotations(podAnnotation)
	if !podCreate {
		return r.client.Update(context.TODO(), pod)
	}
	return nil
}

/*设置Pod的节点亲和性
包括node的亲和性和pod的亲和性
*/
func (r *resourceService) updatePodAffinity(cluster *opengaussv1.OpenGaussCluster, nodes []string, pod *corev1.Pod) {
	affinity := pod.Spec.Affinity
	if affinity == nil {
		affinity = &corev1.Affinity{}
	}
	pod.Spec.NodeSelector = cluster.Spec.Schedule.NodeLabels
	if len(nodes) > 0 {
		affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							corev1.NodeSelectorRequirement{
								Key:      corev1.LabelHostname,
								Operator: corev1.NodeSelectorOpIn,
								Values:   nodes,
							},
						},
					},
				},
			},
		}
	}
	if cluster.GetValidSpec().NetworkClass != opengaussv1.NETWORK_CALICO {
		podAntiAffinityVal := []string{cluster.GetName()}
		affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							metav1.LabelSelectorRequirement{
								Key:      OPENGAUSS_CLUSTER_KEY,
								Operator: metav1.LabelSelectorOpIn,
								Values:   podAntiAffinityVal,
							},
						},
					},
					TopologyKey: corev1.LabelHostname,
				},
			},
		}
	}
	pod.Spec.Affinity = affinity
}

/*
根据网络插件创建Pod时，添加相应的注解
	kube-ovn网络插件：添加ovn.kubernetes.io/ingress_rate和ovn.kubernetes.io/egress_rate注解，不需要带单位，单位为Mbit/s
    其他网络插件： 添加kubernetes.io/ingress-bandwidth和kubernetes.io/egress-bandwidth注解
*/
func (r *resourceService) setPodBandwidth(pod *corev1.Pod, cluster *opengaussv1.OpenGaussCluster) {
	bandWidth := cluster.GetValidSpec().BandWidth
	networkClass := cluster.GetValidSpec().NetworkClass
	if bandWidth != "" {
		bandWidth = utils.GetFixedBandWidth(bandWidth)
		annotation := pod.GetAnnotations()
		if annotation == nil {
			annotation = make(map[string]string)
		}
		if opengaussv1.NETWORK_KUBE_OVN == networkClass {
			annotation[KUBEOVN_BANDWIDTH_INGRESS_KEY] = utils.CalculateBandwidthResourceForKubeovn(bandWidth)
			annotation[KUBEOVN_BANDWIDTH_EGRESS_KEY] = utils.CalculateBandwidthResourceForKubeovn(bandWidth)
		} else {
			annotation[BANDWIDTH_INGRESS_KEY] = bandWidth
			annotation[BANDWIDTH_EGRESS_KEY] = bandWidth
		}
		pod.SetAnnotations(annotation)
	}
}

/*
查看当前og集群下的pod中是否有admin pvc的volume
方法逻辑：
	查询当前cr下的所有pod，遍历pod，查看pod中是否包含admin pvc的volume
返回值
	true： 包含admin pvc
	false： 不包含admin pvc
*/
func (r *resourceService) ClusterPodsIsHaveAdminPvc(cluster *opengaussv1.OpenGaussCluster) bool {
	pods, err := r.FindPodsByCluster(cluster, false)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]查询集群Pod，发生错误", cluster.Namespace, cluster.Name))
		return false
	}
	//判断集群下的pod是否有amdin pvcvolume
	for _, pod := range pods {
		if r.PodIsHaveAdminPvc(pod) {
			return true
		}
	}
	return false
}

/*
查看当前pod中是否包含admin pvc的volume
方法逻辑：
	查看pod中是否包含admin pvc的volume
返回值
	true： 包含admin pvc
	false： 不包含admin pvc
*/
func (r *resourceService) PodIsHaveAdminPvc(pod corev1.Pod) bool {
	volumes := pod.Spec.Volumes
	for _, volume := range volumes {
		if strings.EqualFold(volume.Name, ADMIN_PVC_NAME) {
			return true
		}
	}
	return false
}

/**
查看当前pod是否包含因 FailedAttachVolume和FailedMount原因导致启动失败的Event
方法逻辑：
	根据命名空间及pod名，查询出当前pod的event信息，遍历event，查看event reason是否包含FailedAttachVolume和FailedMount
返回值
	true: 包含
	false： 不包含
*/
func (r *resourceService) IsIncludeFailMountEvents(namespace, clusterName, podName string) (bool, corev1.Event) {
	events, _ := r.KubeClient.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{FieldSelector: "involvedObject.name=" + podName, TypeMeta: metav1.TypeMeta{Kind: "Pod"}})
	for _, item := range events.Items {
		if strings.Contains(strings.ToLower(item.Reason), POD_FAILEDATTACHVOLUME_EVENT) || strings.Contains(strings.ToLower(item.Reason), POD_FAILEDMOUNT_EVENT) {
			r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod: %s  event time: %s ,event message: %s, event Reason : %s ,event action: %s, event source host: %s, event source component :%s, event LastTimestamp:%s ", namespace, clusterName, podName,
				item.EventTime.Time.Format(utils.TIME_FORMAT), item.Message, item.Reason, item.Action, item.Source.Host, item.Source.Component, item.LastTimestamp.Time.Format(utils.TIME_FORMAT)))
			return true, item
		}
	}
	return false, corev1.Event{}
}
func (r *resourceService) GetPodEvents(namespace, clusterName, podName string) string {
	events, _ := r.KubeClient.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{FieldSelector: "involvedObject.name=" + podName, TypeMeta: metav1.TypeMeta{Kind: "Pod"}})
	var eventstr strings.Builder
	eventstr.WriteString(fmt.Sprintf("[%s:%s]Pod %s events:\n ", namespace, clusterName, podName))
	for _, item := range events.Items {
		eventstr.WriteString(fmt.Sprintf("event time: %s ,event message: %s, event Reason : %s ,event action: %s, event source host: %s, event source component :%s, event LastTimestamp: %s  ",
			item.EventTime.Time.Format(utils.TIME_FORMAT), item.Message, item.Reason, item.Action, item.Source.Host, item.Source.Component, item.LastTimestamp.Time.Format(utils.TIME_FORMAT)))
	}
	return eventstr.String()
}

/**
校验当前pod的extendIp正确，修复extendIp与期望不一致的问题
以下情况会触发升级
1.当前pod的extendIp与spec.iplist的extendip不一致，会触发升级操作，删除pod重建
2.当前pod没有extendIp，而spec.iplist中设置了extendip，会触发升级操作，删除pod重建
以下情况不处理
暂不考虑集群设置了extendIp，之后又将extendIp删除的情况
即当前pod有extendIp，而cr的spec.iplist删除了extendip，该情况不会触发升级操作，不会删除pod重建，但pod如果由于其他原因被删除后，下次重建会按照spec中设置，为pod设置对应注解
*/
func (r *resourceService) IsExtendIpMatch(cluster *opengaussv1.OpenGaussCluster, pod corev1.Pod, ipExtendIpMap map[string]string) bool {
	if opengaussv1.NETWORK_KUBE_OVN == cluster.GetValidSpec().NetworkClass {
		//如果spec.iplist中的extendip不为空，再判断当前pod的extendIp是否正确
		if ipExtendIpMap[pod.Status.PodIP] != "" {
			podAnnotation := pod.GetAnnotations()
			attachmentNetworkVal, ok := cluster.Spec.Annotations[utils.ATTACHMENT_NETWORK_ANNOTATION]
			if ok {
				if attachmentNetworkArr, err := utils.GetAttachNetworkArr(attachmentNetworkVal); err == nil {
					attachNetworkName := attachmentNetworkArr[0].Name
					attachNetworkNamespace := attachmentNetworkArr[0].Namespace
					providerName := fmt.Sprintf("%s.%s.ovn", attachNetworkName, attachNetworkNamespace)
					attachNetworkIpAddressKey := fmt.Sprintf(utils.IP_ADDRESS_ANNOTATION_TEMPLATE, providerName)
					attachNetworkSubnetNameKey := fmt.Sprintf(utils.LOGICAL_SWITCH_ANNOTATION_TEMPLATE, providerName)
					extendIpVal, extendIpAnnotation := podAnnotation[attachNetworkIpAddressKey]
					_, extendIpSubnet := podAnnotation[attachNetworkSubnetNameKey]
					if !extendIpAnnotation && !extendIpSubnet {
						r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod %s extendip 不为空，但是缺少extend子网相关注解[%s:%s]", cluster.Namespace, cluster.Name, pod.Name,
							attachNetworkSubnetNameKey, attachNetworkIpAddressKey))
						return false
					}
					//指定附加网卡的子网名称，并移除Pod中的attach_logical_switch_array注解
					if extendIpVal != ipExtendIpMap[pod.Status.PodIP] {
						r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod %s extendip unmatched,expect is %s, actual is %s", cluster.Namespace, cluster.Name, pod.Name,
							ipExtendIpMap[pod.Status.PodIP], podAnnotation[attachNetworkIpAddressKey]))
						return false
					}
				}
			} else {
				r.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod %s extendip 不为空，但是缺少扩展网卡相关注解[%s]", cluster.Namespace, cluster.Name, pod.Name,
					utils.ATTACHMENT_NETWORK_ANNOTATION))
				return false
			}
		}
	}
	return true
}

/**
校验当前cr对应的pod的extendIp正确，修复extendIp与期望不一致的问题
仅cluster是合法的才去校验
*/
func (r *resourceService) ClusterPodsIsExtendIpMatch(cluster *opengaussv1.OpenGaussCluster) bool {
	if cluster.IsValid() && opengaussv1.NETWORK_KUBE_OVN == cluster.GetValidSpec().NetworkClass {
		ipExtendIpMap := cluster.GetIpExtendIpMap()
		pods, _ := r.FindPodsByCluster(cluster, true)
		for _, pod := range pods {
			if !r.IsExtendIpMatch(cluster, pod, ipExtendIpMap) {
				return false
			}
		}
		return true
	} else {
		return true
	}
}
