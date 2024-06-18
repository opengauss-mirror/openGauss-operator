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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	opengaussv1 "opengauss-operator/api/v1"
	"opengauss-operator/controllers/handler"
	"opengauss-operator/utils"
)

const (
	REQUEUE_INTERVAL = 30
)

// OpenGaussClusterReconciler reconciles a OpenGaussCluster object
type OpenGaussClusterReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	syncHandler   handler.SyncHandler
	PollingPeriod int64
}

func NewOpenGaussClusterReconciler(mgr ctrl.Manager, logger logr.Logger, concurrentConciler int, watchNamespaces, excludeNamespaces string, reconcilePollingPeriod int64) (reconcile.Reconciler, error) {
	reconciler := &OpenGaussClusterReconciler{Client: mgr.GetClient(), Log: logger, Scheme: mgr.GetScheme()}
	reconciler.PollingPeriod = reconcilePollingPeriod
	reconciler.syncHandler = handler.NewSyncHandler(reconciler.Client, reconciler.Log, mgr.GetEventRecorderFor("opengauss-operator"))
	option := controller.Options{
		MaxConcurrentReconciles: concurrentConciler,
	}
	nsPred := utils.NewWatchScopePredicate(watchNamespaces, excludeNamespaces)
	hcPred := utils.NewHashCodePredicate(concurrentConciler)
	err := ctrl.NewControllerManagedBy(mgr).For(&opengaussv1.OpenGaussCluster{}).WithOptions(option).WithEventFilter(nsPred).WithEventFilter(hcPred).Complete(reconciler)
	return reconciler, err
}

// +kubebuilder:rbac:groups=opengauss.sig,resources=opengaussclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=opengauss.sig,resources=opengaussclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=opengauss.sig,resources=opengaussclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events;pods;pods/exec;persistentvolumeclaims;persistentvolumes;configmaps;secrets;services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenGaussCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *OpenGaussClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("opengausscluster", req.NamespacedName)

	//查询CR
	cluster := &opengaussv1.OpenGaussCluster{}
	err := r.Get(context.TODO(), req.NamespacedName, cluster)
	if err != nil {
		//如果CR不存在，则结束
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	//如果CR已被删除，进行相应资源的清理操作
	if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		r.Log.Info(fmt.Sprintf("[%s:%s]删除集群", cluster.Namespace, cluster.Name))

		//添加资源清理操作

		r.Log.Info(fmt.Sprintf("[%s:%s]删除集群完成", cluster.Namespace, cluster.Name))
		return reconcile.Result{}, nil
	}

	//如果校验结果无需操作，则结束
	instance := cluster.DeepCopy()
	valid, doReconcile := r.syncHandler.Validate(instance)
	if !doReconcile {
		return reconcile.Result{}, nil
	}
	if instance.Spec.Schedule.PollingPeriod == 0 {
		instance.Spec.Schedule.PollingPeriod = r.PollingPeriod
	}
	//如果校验通过，对空属性填充默认值
	if valid {
		if err := r.syncHandler.SetDefault(instance); err != nil {
			return reconcile.Result{RequeueAfter: time.Second * time.Duration(cluster.Spec.Schedule.PollingPeriod)}, err
		}
	}

	//根据CR Spec配置集群
	if err := r.syncHandler.SyncCluster(instance); err != nil {
		r.Log.Error(err, fmt.Sprintf("[%s:%s]配置集群发生错误，将于%d秒后重试", instance.Namespace, instance.Name, REQUEUE_INTERVAL))
		time.Sleep(time.Second * REQUEUE_INTERVAL)
		return reconcile.Result{RequeueAfter: time.Second * time.Duration(cluster.Spec.Schedule.PollingPeriod)}, err
	}
	return reconcile.Result{RequeueAfter: time.Second * time.Duration(cluster.Spec.Schedule.PollingPeriod)}, nil
}
