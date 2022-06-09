# 常见故障诊断
## 诊断方式
### Operator 看状态查日志
operator默认采用Deployments方式部署，默认部署在opengauss-operator-system命名空间下
查看operator的Deployments状态及其对应pod的状态
> 查看operator的Deployments状态
> kubectl get deployments.apps  -n \<namaspacename>
> 查看operator的pod的状态
> kubectl get pod  -n \<namaspacename>

示例:
```bash
#查看operator的Deployments状态
$ kubectl get deployments.apps  -n opengauss-operator-system 
NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
opengauss-operator-controller-manager   1/1     1            1           3m41s
# 查看operator的pod的状态
kubectl get pod  -n opengauss-operator-system                  
NAME                                                     READY   STATUS    RESTARTS   AGE
opengauss-operator-controller-manager-5d7c58c7d5-8j4cz   1/1     Running   0          6m25s
```

查看operator日志
> kubectl logs -f -n \<namespacename> \<podname>

```bash
#查看operator pod信息
$ kubectl get pod  -n opengauss-operator-system                  
NAME                                                     READY   STATUS    RESTARTS   AGE
opengauss-operator-controller-manager-5d7c58c7d5-8j4cz   1/1     Running   0          6m25s
# 查看operator的  日志
kubectl logs -f -n opengauss-operator-system  opengauss-operator-controller-manager-5d7c58c7d5-8j4cz 
···
2022-06-06T09:28:59.721Z        INFO    controllers.OpenGaussCluster    [og:test2]开始处理集群
2022-06-06T09:28:59.722Z        DEBUG   controllers.OpenGaussCluster    [og:og-test2-pod-172x16x0x4]执行命令：bash /gauss/files/K8SChkRepl.sh
2022-06-06T09:29:00.875Z        DEBUG   controllers.OpenGaussCluster    [og:test2]位于Pod og-test2-pod-172x16x0x4上的数据库状态：[Local Role: Primary, Process exist: true, Connection available: true, DB state normal: true, Maintenance: false, Backup status: no data, Restore status: no data, Static Connections: 1, Detail Information: Normal, ]
2022-06-06T09:29:00.876Z        DEBUG   controllers.OpenGaussCluster    [og:test2]集群状态正常
2022-06-06T09:29:00.876Z        DEBUG   controllers.OpenGaussCluster    [og:og-test2-pod-172x16x0x4]执行命令：bash /gauss/files/K8SChkRepl.sh
2022-06-06T09:29:02.031Z        DEBUG   controllers.OpenGaussCluster    [og:test2]位于Pod og-test2-pod-172x16x0x4上的数据库状态：[Local Role: Primary, Process exist: true, Connection available: true, DB state normal: true, Maintenance: false, Backup status: no data, Restore status: no data, Static Connections: 1, Detail Information: Normal, ]
2022-06-06T09:29:02.031Z        DEBUG   controllers.OpenGaussCluster    [og:og-test2-pod-172x16x0x4]执行命令：gs_ctl query -D /gaussdata/openGauss/db1
2022-06-06T09:29:02.489Z        INFO    controllers.OpenGaussCluster    [og:test2]集群处理完成
```
### OG 看状态查日志
查看opengauss集群的状态 
> kubectl get opengaussclusters.opengauss.sig  -n \<namespacename> \<crName>

示例
```bash
kubectl get opengaussclusters.opengauss.sig  -n og            
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogtest   primary   500m   2Gi      30001       30002        5432      ready   6d
test2    primary   500m   2Gi      30003       30004        5432      ready   5d
```
查看opengauss集群的状态的详情
>  kubectl describe opengaussclusters.opengauss.sig  -n \<namespacename> \ <crName>

示例：
```bash
$ kubectl describe opengaussclusters.opengauss.sig  -n og ogtest
Name:         ogtest
Namespace:    og
Labels:       <none>
Annotations:  <none>
API Version:  opengauss.sig/v1
Kind:         OpenGaussCluster
Metadata:
  ...
Spec:
  Archivelogpath:  /data/k8slocalogarchive
  Backuppath:      /data/k8slocalxbk
  Config:
    most_available_sync:  on
  Cpu:                    500m
  Dbport:                 5432
  Filebeatconfig:         opengauss-filebeat-config
  Image:                  opengauss.sig/opengauss/container:2.0.0.v7
  Iplist:
    Ip:        172.16.0.2
    Nodename:  node1
    Ip:        172.16.0.5
    Nodename:  node2
  Localrole:   primary
  Memory:      2Gi
  Readport:    30001
  Schedule:
    Grace Period:          30
    Mostavailabletimeout:  60
    Process Timeout:       300
    Toleration:            300
  Scriptconfig:            opengauss-script-config
  Sidecarcpu:              500m
  Sidecarmemory:           1Gi
  Sidecarstorage:          1Gi
  Storage:                 8Gi
  Storageclass:            topolvm-provisioner
  Writeport:               30002
...
Events:
  Type     Reason            Age   From                Message
  ----     ------            ----  ----                -------
  Normal   SetMostAvailable  46m   opengauss-operator  2022-06-06 14:21:08 设置最大可用为"ON"
  Warning  Recorvering       46m   opengauss-operator  2022-06-06 14:21:08 恢复集群
  Normal   SetMostAvailable  46m   opengauss-operator  2022-06-06 14:21:52 设置最大可用为"OFF"
  Normal   Ready             46m   opengauss-operator  2022-06-06 14:21:57 集群正常
```

查看 og集群下pod的日志
> kubectl logs -f -n  \<namespacename> \<podName>

```bash
$ kubectl logs -f -n og og-ogtest-pod-172x16x0x2
...
error: a container name must be specified for pod og-ogtest-pod-172x16x0x2, choose one of: [og sidecar] or one of the init containers: [initenv]
$ kubectl logs -f -n og og-ogtest-pod-172x16x0x2 -c og

2022-06-06 14:21:23.250 [unknown] [unknown] localhost 139795642226432 0 0 [BACKEND] LOG:  create gaussdb state file success: db state(STARTING_STATE), server mode(Pending)
2022-06-06 14:21:23.293 [unknown] [unknown] localhost 139795642226432 0 0 [BACKEND] LOG:  max_safe_fds = 976, usable_fds = 1000, already_open = 14
The core dump path is an invalid directory
2022-06-06 14:21:23.299 [unknown] [unknown] localhost 139795642226432 0 0 [BACKEND] LOG:  the configure file /gauss/openGauss/app/etc/gscgroup_omm.cfg doesn't exist or the size of configure file has changed. Please create it by root user!
2022-06-06 14:21:23.299 [unknown] [unknown] localhost 139795642226432 0 0 [BACKEND] LOG:  Failed to parse cgroup config file.
.
[2022-06-06 14:21:24.475][25][][gs_ctl]:  done
[2022-06-06 14:21:24.475][25][][gs_ctl]: server started (/gaussdata/openGauss/db1)
```