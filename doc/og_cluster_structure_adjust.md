# openGauss集群架构调整
## 扩容
opengauss集群的扩容是通过修改CR的iplist属性来实现的，扩容即新增iplist的一个元素，反之缩容即删除iplist中已存在的一个元素
```yaml
iplist:
  - ip: 172.16.0.2
    nodename: node1
  - ip: 172.16.0.3
    nodename: node2
```

示例: k8s的og命名空间下有单节点og集群`ogtest`，此时集群初始状态为ready：
```bash
$ kubectl get opengaussclusters.opengauss.sig -n og ogtest 
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogtest   primary   500m   2Gi      30001       30002        5432      ready   151m
```
其spec中的iplist如下：
```bash
$ kubectl describe opengaussclusters.opengauss.sig -n og ogtest 
  Spec:
  Archivelogpath:  /data/k8slocalogarchive
  Backuppath:      /data/k8slocalxbk
  Cpu:             500m
  Dbport:          5432
  Filebeatconfig:  opengauss-filebeat-config
  Image:           opengauss.sig/opengauss/container:2.0.0.v7
  Iplist:
    Ip:        172.16.0.2
    Nodename:  node3
  Localrole:   primary
  Memory:      2Gi
  Readport:    30001
```
ogtest集群的pod如下：
```bash
$ kubectl get pod -n og --show-labels
NAME                       READY   STATUS    RESTARTS   AGE    LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          154m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primarybels

```
修改spec中iplist，新增ip为172.16.0.5，nodename为node2的元素,即扩容一个节点
```bash
cat <<EOF |  kubectl apply -f -
apiVersion: opengauss.sig/v1
kind: OpenGaussCluster
metadata:
  name: ogtest
  namespace: og
spec:
  cpu: 500m
  dbport: 5432
  image: opengauss.sig/opengauss/container:2.0.0.v7
  memory: 2Gi
  storage: 5Gi
  storageclass: topolvm-provisioner
  readport: 30001
  writeport: 30002
  iplist:
  - ip: 172.16.0.2
    nodename: node1
  - ip: 172.16.0.5
    nodename: node2
EOF
```
重新apply后，CR state由ready变为updating，如下：
```bash
$ kubectl get opengaussclusters.opengauss.sig  -n og 
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogtest   primary   500m   2Gi      30001       30002        5432      ready   122m
```
一段时候后，扩容的节点172.16.0.5就会成功部署，扩容成功后，cr状态会恢复为ready。在整个扩容过程中，CR的状态和ogtest集群(CR)的pod状态如下：
```bash
#cr 状态变化如下
$ kubectl get opengaussclusters.opengauss.sig  -n og  -w
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogtest   primary   500m   2Gi      30001       30002        5432      ready   122m
ogtest   primary   500m   2Gi      30001       30002        5432      ready   123m
ogtest   primary   500m   2Gi      30001       30002        5432      ready   123m
ogtest   primary   500m   2Gi      30001       30002        5432      ready   123m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   123m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   125m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   125m
ogtest   primary   500m   2Gi      30001       30002        5432      ready      127m
# ogtest集群的pod状态如下
$ kubectl get pod -n og --show-labels -o wide -w
NAME                       READY   STATUS    RESTARTS   AGE   IP           NODE       NOMINATED NODE   READINESS GATES   LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          93m   172.16.0.2   node1   <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x5   0/2     Pending   0          1s    <none>       <none>     <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Pending   0          4s    <none>       node2   <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Init:0/1   0          4s    <none>       node2    <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Init:0/1   0          14s   <none>       node2    <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Init:0/1   0          15s   172.16.0.5   node2    <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     PodInitializing   0          74s   172.16.0.5   node2    <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   1/2     Running           0          75s   172.16.0.5   node2   <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   2/2     Running           0          2m14s   172.16.0.5   node2    <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   2/2     Running           0          2m21s   172.16.0.5   node2    <none>           <none>            app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
```
验证扩容是否成功，通过查看ogtest集群的pod信息以及进入容器内执行`gs_ctl`命令
```bash
# ogtest集群的pod最终信息如下
$  kubectl get pod -n og --show-labels -w
NAME                       READY   STATUS    RESTARTS   AGE     LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          99m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x5   2/2     Running   0          6m11s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
#进入primary og-ogtest-pod-172x16x0x2 的pod中，执行gs_ctl查看集群详情
kubectl exec -ti -n og og-ogtest-pod-172x16x0x2 -c og -- gs_ctl query -D gaussdata/openGauss/db1
[2022-05-31 16:17:54.426][37694][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 32150
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/40006B8
        sender_write_location          : 0/40006B8
        sender_flush_location          : 0/40006B8
        sender_replay_location         : 0/40006B8
        receiver_received_location     : 0/40006B8
        receiver_write_location        : 0/40006B8
        receiver_flush_location        : 0/40006B8
        receiver_replay_location       : 0/40006B8
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.2:5434-->172.16.0.5:39028

 Receiver info:      
No information 
```
结果如上，代表扩容操作成功。

## 缩容
示例：k8s og命名空间下已有一个一主两从的og集群`ogtest`，集群初始状态为ready
```bash
kubectl get opengaussclusters.opengauss.sig -n og ogtest 
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogtest   primary   500m   2Gi      30001       30002        5432      ready   28m
```
其spec中的iplist如下：
```bash
$ kubectl describe opengaussclusters.opengauss.sig -n og ogtest 
...
  Spec:
    Archivelogpath:  /data/k8slocalogarchive
    Backuppath:      /data/k8slocalxbk
    Cpu:             500m
    Dbport:          5432
    Filebeatconfig:  opengauss-filebeat-config
    Image:           opengauss.sig/opengauss/container:2.0.0.v7
    Iplist:
      Ip:        172.16.0.2
      Nodename:  node1
      Ip:        172.16.0.3
      Nodename:  node2
      Ip:        172.16.0.4
      Nodename:  node3
    Localrole:   primary
    Memory:      2Gi
    Readport:    30001
...
```
ogtest集群的pod如下：
```bash
kubectl get pod -n og --show-labels
NAME                       READY   STATUS    RESTARTS   AGE   LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          31m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x3   2/2     Running   0          31m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x4   2/2     Running   0          31m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
```
修改spec中iplist，删除ip为172.16.0.4的元素，即缩容一个节点
```bash
cat <<EOF |  kubectl apply -f -
apiVersion: opengauss.sig/v1
kind: OpenGaussCluster
metadata:
  name: ogtest
  namespace: og
spec:
  cpu: 500m
  dbport: 5432
  image: opengauss.sig/opengauss/container:2.0.0.v7
  memory: 2Gi
  storage: 5Gi
  storageclass: topolvm-provisioner
  readport: 30001
  writeport: 30002
  iplist:
  - ip: 172.16.0.2
    nodename: node1
  - ip: 172.16.0.3
    nodename: node2
EOF
```
重新apply后，CR state由ready变为updating,如下：
```bash
$ kubectl get opengaussclusters.opengauss.sig -n og ogtest
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE      AGE
ogtest   primary   500m   2Gi      30001       30002        5432      updating   36m
```
一段时候后，缩容的节点172.16.0.4就会被删除，删除成功后，cr状态会恢复为ready
整个缩容过程中，CR的状态和ogtest集群(CR)的pod状态如下：
```bash
#cr 状态变化如下
$ kubectl get opengaussclusters.opengauss.sig -n og ogtest -w
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE      AGE
ogtest   primary   500m   2Gi      30001       30002        5432      updating   37m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   37m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   37m
ogtest   primary   500m   2Gi      30001       30002        5432      ready      37m
# ogtest集群的pod状态如下
$ kubectl get pod -n og --show-labels -w
NAME                       READY   STATUS    RESTARTS   AGE   LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          33m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x3   2/2     Running   0          33m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x4   2/2     Running   0          33m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x4   2/2     Terminating   0          36m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x4   0/2     Terminating   0          36m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x4   0/2     Terminating   0          37m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x4   0/2     Terminating   0          37m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   2/2     Running       0          38m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x2   2/2     Running       0          38m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
#
```
通过查看ogtest集群的pod信息验证缩容是否成功
```bash
# ogtest集群的pod最终信息如下
$ kubectl get pod -n og --show-labels -w
NAME                       READY   STATUS    RESTARTS   AGE   LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          44m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x3   2/2     Running   0          44m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
```
进入容器内执行`gs_ctl`命令
```bash
#进入primary og-ogtest-pod-172x16x0x2 的pod中，执行gs_ctl查看集群详情
$ kubectl exec -ti -n og og-ogtest-pod-172x16x0x2 -c og -- gs_ctl query -D gaussdata/openGauss/db1
[2022-05-31 11:59:23.017][78130][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 59312
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/4003290
        sender_write_location          : 0/4003290
        sender_flush_location          : 0/4003290
        sender_replay_location         : 0/4003290
        receiver_received_location     : 0/4003290
        receiver_write_location        : 0/4003290
        receiver_flush_location        : 0/4003290
        receiver_replay_location       : 0/4003178
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.2:5434-->172.16.0.3:47704

 Receiver info:      
No information 
```
结果如上，代表缩容操作成功。 **注意缩容的节点，会保留其PVC，需要主动删除**