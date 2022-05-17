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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IpNodeEntry defines a ip and name of the node which the ip located
type IpNodeEntry struct {
	Ip       string `json:"ip"`
	NodeName string `json:"nodename"`
}

// OpenGaussClusterSpec defines the desired state of OpenGaussCluster
type OpenGaussClusterSpec struct {
	ReadPort       int32             `json:"readport,omitempty"`       //读端口
	WritePort      int32             `json:"writeport,omitempty"`      //写端口
	DBPort         int32             `json:"dbport,omitempty"`         //数据库端口
	Image          string            `json:"image,omitempty"`          //镜像地址
	SidecarImage   string            `json:"sidecarimage,omitempty"`   //sidecar镜像地址
	LocalRole      string            `json:"localrole,omitempty"`      //集群角色 {primary | standby}
	Cpu            string            `json:"cpu,omitempty"`            //CPU
	Memory         string            `json:"memory,omitempty"`         //内存
	Storage        string            `json:"storage,omitempty"`        //存储
	BandWidth      string            `json:"bandwidth,omitempty"`      //带宽
	SidecarCpu     string            `json:"sidecarcpu,omitempty"`     //sidecar CPU
	SidecarMemory  string            `json:"sidecarmemory,omitempty"`  //sidecar 内存
	SidecarStorage string            `json:"sidecarstorage,omitempty"` //sidecar 存储
	IpList         []IpNodeEntry     `json:"iplist,omitempty"`         //IP-节点信息，描述每个POD将被分配的IP以及所部署的worknode名称
	RemoteIpList   []string          `json:"remoteiplist,omitempty"`   //同城节点的IP信息
	BackupPath     string            `json:"backuppath,omitempty"`     //本地备份路径
	ArchiveLogPath string            `json:"archivelogpath,omitempty"` //本地归档路径
	HostpathRoot   string            `json:"hostpathroot,omitempty"`   //本地存储根路径，使用本地存储时填写
	StorageClass   string            `json:"storageclass,omitempty"`   //storageclass，使用CSI时填写
	Maintenance    bool              `json:"maintenance,omitempty"`    //集群维护模式
	Config         map[string]string `json:"config,omitempty"`         //数据库配置参数
	ScriptConfig   string            `json:"scriptconfig,omitempty"`   //执行脚本的配置
	FilebeatConfig string            `json:"filebeatconfig,omitempty"` //Filebeat配置CM
	RestoreFile    string            `json:"restorefile,omitempty"`    //数据恢复文件
	Schedule       ScheduleConfig    `json:"schedule,omitempty"`
}

type ScheduleConfig struct {
	ProcessTimeout       int32 `json:"processTimeout,omitempty"`
	GracePeriod          int32 `json:"gracePeriod,omitempty"`
	Toleration           int32 `json:"toleration,omitempty"`
	MostAvailableTimeout int32 `json:"mostavailabletimeout,omitempty"`
}

type OpenGaussClusterState string

const (
	OpenGaussClusterStateReady    OpenGaussClusterState = "ready"
	OpenGaussClusterStateCreate   OpenGaussClusterState = "creating"
	OpenGaussClusterStateUpdate   OpenGaussClusterState = "updating"
	OpenGaussClusterStateFailed   OpenGaussClusterState = "failed"
	OpenGaussClusterStateMaintain OpenGaussClusterState = "maintaining"
	OpenGaussClusterStateInvalid  OpenGaussClusterState = "invalid"
	OpenGaussClusterStateRecover  OpenGaussClusterState = "recovering"
	OpenGaussClusterStateRestore  OpenGaussClusterState = "restoring"
)

type OpenGaussClusterConditionType string

const (
	OpenGaussClusterResourceReady  OpenGaussClusterConditionType = "ResourceReady"
	OpenGaussClusterInstancesReady OpenGaussClusterConditionType = "InstancesReady"
	OpenGaussClusterServiceReady   OpenGaussClusterConditionType = "ServiceReady"
)

type OpenGaussClusterCondition struct {
	Type           OpenGaussClusterConditionType `json:"type,omitempty"`
	Status         corev1.ConditionStatus        `json:"status,omitempty"`
	LastUpdateTime string                        `json:"lastUpdateTime,omitempty"`
	Message        string                        `json:"message,omitempty"`
}

type SyncState struct {
	IP       string `json:"ip,omitempty"`
	Percent  int    `json:"percent,omitempty"`
	State    string `json:"state,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

type RestorePhase string

const (
	RestorePhasePrepare   RestorePhase = "Prepare"
	RestorePhaseRunning   RestorePhase = "Running"
	RestorePhaseSucceeded RestorePhase = "Succeeded"
)

// OpenGaussClusterStatus defines the observed state of OpenGaussCluster
type OpenGaussClusterStatus struct {
	State          OpenGaussClusterState       `json:"state,omitempty"`
	Primary        string                      `json:"primary,omitempty"`
	Message        string                      `json:"message,omitempty"`
	Conditions     []OpenGaussClusterCondition `json:"conditions,omitempty"`
	Spec           OpenGaussClusterSpec        `json:"spec,omitempty"`
	PodState       map[string]string           `json:"podstate,omitempty"`
	SyncStates     []SyncState                 `json:"syncState,omitempty"`
	RestorePhase   RestorePhase                `json:"restore,omitempty"`
	LastUpdateTime string                      `json:"lastUpdateTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={ogc}
// +kubebuilder:printcolumn:name="Role",type="string",JSONPath=".spec.localrole",description="OpenGaussCluster LocalRole"
// +kubebuilder:printcolumn:name="CPU",type="string",JSONPath=".spec.cpu",description="OpenGaussCluster CPU Limit"
// +kubebuilder:printcolumn:name="Memory",type="string",JSONPath=".spec.memory",description="OpenGaussCluster Memory Limit"
// +kubebuilder:printcolumn:name="Read Port",type="integer",JSONPath=".spec.readport",description="OpenGaussCluster Read Service Port"
// +kubebuilder:printcolumn:name="Write Port",type="integer",JSONPath=".spec.writeport",description="OpenGaussCluster Write Service Port"
// +kubebuilder:printcolumn:name="DB Port",type="integer",JSONPath=".spec.dbport",description="OpenGaussCluster DB Port"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="OpenGaussCluster state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OpenGaussCluster is the Schema for the opengaussclusters API
type OpenGaussCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenGaussClusterSpec   `json:"spec,omitempty"`
	Status OpenGaussClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenGaussClusterList contains a list of OpenGaussCluster
type OpenGaussClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenGaussCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenGaussCluster{}, &OpenGaussClusterList{})
}
