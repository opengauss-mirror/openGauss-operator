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
	"path/filepath"

	"opengauss-operator/utils"
)

const (
	NODEPORT_MIN = 30000
	NODEPORT_MAX = 32767
)

func (cluster *OpenGaussCluster) Validate() error {
	errorMessage := make([]string, 0)
	if cluster.Spec.Image == "" {
		errorMessage = append(errorMessage, "属性\"Image\"不能为空")
	}
	if !cluster.IsNew() {
		if cluster.Spec.DBPort != cluster.Status.Spec.DBPort {
			errorMessage = append(errorMessage, "属性\"DBPort\"不支持修改")
		}
		if cluster.Spec.HostpathRoot != cluster.Status.Spec.HostpathRoot {
			errorMessage = append(errorMessage, fmt.Sprintf("属性\"HostpathRoot\"不支持修改"))
		}
		if cluster.Spec.StorageClass != cluster.Status.Spec.StorageClass {
			errorMessage = append(errorMessage, fmt.Sprintf("属性\"StorageClass\"不支持修改"))
		}
	}
	role := cluster.Spec.LocalRole
	if role != "" && role != LOCAL_ROLE_PRIMARY && role != LOCAL_ROLE_STANDBY {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"LocalRole\"的值\"%s\"无效，可选的值为\"primary\"或\"standby\"", cluster.Spec.LocalRole))
	}
	if cluster.Spec.ReadPort < NODEPORT_MIN || cluster.Spec.ReadPort > NODEPORT_MAX {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"ReadPort\"的值\"%d\"无效，端口的取值范围是30000-32767", cluster.Spec.ReadPort))
	}
	if cluster.Spec.WritePort < NODEPORT_MIN || cluster.Spec.WritePort > NODEPORT_MAX {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"WritePort\"的值\"%d\"无效，端口的取值范围是30000-32767", cluster.Spec.WritePort))
	}
	if !utils.ValidateResource(cluster.Spec.Cpu) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"CPU\"的值\"%s\"无效", cluster.Spec.Cpu))
	} else if cluster.Spec.Cpu != "" && utils.CompareQuantity(cluster.Spec.Cpu, DB_CPU_REQ) < 0 {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"CPU\"的值\"%s\"小于最低要求\"%s\"", cluster.Spec.Cpu, DB_CPU_REQ))
	}
	if !utils.ValidateResource(cluster.Spec.Memory) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"Memory\"的值\"%s\"无效", cluster.Spec.Memory))
	} else if cluster.Spec.Memory != "" && utils.CompareQuantity(cluster.Spec.Memory, DB_MEM_REQ) < 0 {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"Memory\"的值\"%s\"小于最低要求\"%s\"", cluster.Spec.Memory, DB_MEM_REQ))
	}
	//存储校验
	//存储的值不能小于系统最小值，也不能小于之前设置的值
	//SidecarStorage逻辑相同
	if !utils.ValidateResource(cluster.Spec.Storage) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"Storage\"的值\"%s\"无效", cluster.Spec.Storage))
	} else if cluster.Spec.Storage != "" {
		if utils.CompareQuantity(cluster.Spec.Storage, DB_STORAGE_REQ) < 0 {
			errorMessage = append(errorMessage, fmt.Sprintf("属性\"Storage\"的值\"%s\"小于最低要求\"%s\"", cluster.Spec.Storage, DB_STORAGE_REQ))
		} else if !cluster.IsNew() && utils.CompareQuantity(cluster.Spec.Storage, cluster.Status.Spec.Storage) < 0 {
			errorMessage = append(errorMessage, fmt.Sprintf("属性\"Storage\"的值\"%s\"小于之前设置的值\"%s\"", cluster.Spec.Storage, cluster.Status.Spec.Storage))
		}
	}
	if !utils.ValidateResource(cluster.Spec.BandWidth) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"BandWidth\"的值\"%s\"无效", cluster.Spec.BandWidth))
	}
	if !utils.ValidateResource(cluster.Spec.SidecarCpu) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"SidecarCpu\"的值\"%s\"无效", cluster.Spec.SidecarCpu))
	} else if cluster.Spec.SidecarCpu != "" && utils.CompareQuantity(cluster.Spec.SidecarCpu, SIDECAR_CPU_REQ) < 0 {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"SidecarCpu\"的值\"%s\"小于最低要求\"%s\"", cluster.Spec.SidecarCpu, SIDECAR_CPU_REQ))
	}
	if !utils.ValidateResource(cluster.Spec.SidecarMemory) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"SidecarMemory\"的值\"%s\"无效", cluster.Spec.SidecarMemory))
	} else if cluster.Spec.SidecarMemory != "" && utils.CompareQuantity(cluster.Spec.SidecarMemory, SIDECAR_MEM_REQ) < 0 {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"SidecarMemory\"的值\"%s\"小于最低要求\"%s\"", cluster.Spec.SidecarMemory, SIDECAR_MEM_REQ))
	}
	if !utils.ValidateResource(cluster.Spec.SidecarStorage) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"SidecarStorage\"的值\"%s\"无效", cluster.Spec.SidecarStorage))
	} else if cluster.Spec.SidecarStorage != "" {
		if utils.CompareQuantity(cluster.Spec.SidecarStorage, SIDECAR_STORAGE_REQ) < 0 {
			errorMessage = append(errorMessage, fmt.Sprintf("属性\"SidecarStorage\"的值\"%s\"小于最低要求\"%s\"", cluster.Spec.SidecarStorage, SIDECAR_STORAGE_REQ))
		} else if !cluster.IsNew() && utils.CompareQuantity(cluster.Spec.SidecarStorage, cluster.Status.Spec.SidecarStorage) < 0 {
			errorMessage = append(errorMessage, fmt.Sprintf("属性\"SidecarStorage\"的值\"%s\"小于之前设置的值\"%s\"", cluster.Spec.SidecarStorage, cluster.Status.Spec.SidecarStorage))
		}
	}
	//本地存储根路径与StorageClass，只能指定其中之一
	if cluster.Spec.HostpathRoot != "" && cluster.Spec.StorageClass != "" {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"HostpathRoot\"和\"StorageClass\"不能同时设置"))
	}
	//基于Hostpath的属性，如本地存储根路径、备份路径、归档路径，都必须是绝对路径
	if cluster.Spec.HostpathRoot != "" && !filepath.IsAbs(cluster.Spec.HostpathRoot) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"HostpathRoot\"必须是绝对路径"))
	}
	if cluster.Spec.BackupPath != "" && !filepath.IsAbs(cluster.Spec.BackupPath) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"BackupPath\"必须是绝对路径"))
	}
	if cluster.Spec.ArchiveLogPath != "" && !filepath.IsAbs(cluster.Spec.ArchiveLogPath) {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"ArchiveLogPath\"必须是绝对路径"))
	}
	if len(cluster.Spec.IpList) == 0 {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"IpList\"不能为空"))
	} else {
		for _, entry := range cluster.Spec.IpList {
			if !utils.ValidateIp(entry.Ip) {
				errorMessage = append(errorMessage, fmt.Sprintf("IP值\"%s\"无效", entry.Ip))
			} else if entry.NodeName == "" {
				errorMessage = append(errorMessage, fmt.Sprintf("IP\"%s\"对应的\"NodeName\"值不能为空", entry.Ip))
			}
		}
	}
	if len(cluster.Spec.RemoteIpList) > 0 {
		for _, ip := range cluster.Spec.RemoteIpList {
			if !utils.ValidateIp(ip) {
				errorMessage = append(errorMessage, fmt.Sprintf("IP值\"%s\"无效", ip))
			}
		}
	}
	//同城集群的远程地址指向主集群的IpList，必须填写
	if cluster.Spec.LocalRole == LOCAL_ROLE_STANDBY && len(cluster.Spec.RemoteIpList) == 0 {
		errorMessage = append(errorMessage, fmt.Sprintf("属性\"RemoteIpList\"不能为空"))
	}
	//OpenGauss数据库参数中类型为internal的配置参数，不允许修改
	//详见utils.dbproperties.go
	if !cluster.IsNew() {
		internalProps := utils.GetInternalProperties()
		for key, _ := range cluster.Spec.Config {
			if internalProps.Contains(key) {
				errorMessage = append(errorMessage, fmt.Sprintf("数据库配置参数\"%s\"不支持修改", key))
			}
		}
	}
	if len(errorMessage) != 0 {
		return fmt.Errorf("[%s:%s]规约校验失败，原因：%s", cluster.Namespace, cluster.Name, utils.StringArrayToString(errorMessage))
	}
	return nil
}
