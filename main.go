/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"
	"strconv"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	opengaussv1 "opengauss-operator/api/v1"
	"opengauss-operator/controllers"
	"opengauss-operator/utils"
	// +kubebuilder:scaffold:imports
)

const (
	LEADER_KEY = "9e66c0cd.sig"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = opengaussv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var concurrentReconcile string
	var enableLeaderElection bool
	var watchNamespaces string
	var excludeNamespaces string
	var pollingPeriod string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&concurrentReconcile, "max-concurrent-reconcile", "5", "The max concurrent reconcile count.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&watchNamespaces, "watch-namespaces", "", "The namespaces that will be managed by current operator.")
	flag.StringVar(&excludeNamespaces, "exclude-namespaces", "", "The namespaces that will not be managed by current operator.")
	flag.StringVar(&pollingPeriod, "pollingPeriod", "", "The Period of reconcile polling, that is global setting.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   getLeaderElectionID(watchNamespaces),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if enableLeaderElection {
		os.Unsetenv(utils.ENV_TOTAL_KEY)
	}
	concurrent, _ := strconv.Atoi(concurrentReconcile)
	reconcilePollingPeriod, _ := strconv.ParseInt(pollingPeriod, 10, 64)
	_, err = controllers.NewOpenGaussClusterReconciler(mgr, ctrl.Log.WithName("controllers").WithName("OpenGaussCluster"), concurrent,
		watchNamespaces, excludeNamespaces, reconcilePollingPeriod)

	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenGaussCluster")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getLeaderElectionID(watchNamespaces string) string {
	leaderKeyPrefix := ""
	if watchNamespaces != "" {
		namespaces := utils.StringToSet(watchNamespaces)
		leaderKeyPrefix = namespaces.ToArray()[0] + "."
	}
	return leaderKeyPrefix + LEADER_KEY
}
