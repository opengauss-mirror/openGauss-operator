# 同城双集群部署
og集群部署在两个k8s上，每个k8s各有一个节点，组成HA集群，前提需要**保证两个k8s集群网络的连通性**，其中：
1. 部署在本地集群的localrole应设置为primary，同城集群的localrole为standby
2. 同城双集群部署与单集群部署的区别除了localrole的属性的值设置外，还包括**remoteiplist**属性
3. 本地集群和同城集群CR的yaml文件中需要分别增加对方集群的Pod Ip.
先部署本地集群，在部署同城集群

示例：部署一个同城双集群`test2`。本地集群部署一个节点，localrole为**primary**
```yaml
apiVersion: opengauss.sig/v1
kind: OpenGaussCluster
metadata:
  name: test2
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
  localrole: primary
  iplist:  
  - ip: 172.16.0.4
    nodename: node1
  #同城集群的Pod IP,按实际情况填充
  remoteiplist:
  - 172.16.0.4
```
同城集群也部署一个节点，localrole为**standby**
```yaml
apiVersion: opengauss.sig/v1
kind: OpenGaussCluster
metadata:
  name: test2
  namespace: og
spec:
  cpu: 500m
  dbport: 5432
  hostpathroot: ''
  image: opengauss.sig/opengauss/container:2.0.0.v7
  iplist:
  - ip: 172.16.0.3
    nodename: node12
  memory: 2Gi
  readport: 30001
  storage: 5Gi
  storageclass: topolvm-provisioner
  writeport: 30002
  localrole: standby
  remoteiplist:
  - 172.16.0.4
```
分别在两个k8s上查询应用的部署状态: `kubectl get opengaussclusters.opengauss.sig -n <namespace name >  <cr name>`

输出结果类似如下：
```bash
# 本地集群的状态如下
kubectl get opengaussclusters.opengauss.sig -n og 
NAME         ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
test2        primary   500m   2Gi      30003       30004        5432      creating   31s

#同城集群的状态如下：
$ kubectl get opengaussclusters.opengauss.sig -n og -w 
NAME         ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
test2        standby   500m   2Gi      30003       30004        5432      creating   0s
```

直到 同城和本地集群 STATE 都为ready:
```bash
# 本地集群所在k8s查询状态如下
$ kubectl get opengaussclusters.opengauss.sig -n og 
NAME         ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
test2        primary   500m   2Gi      30003       30004        5432      ready   18m

# 同城集群所在k8s查询状态如下
$ kubectl get opengaussclusters.opengauss.sig -n og 
NAME         ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
test2        standby   500m   2Gi      30003       30004        5432      ready   5m6s
```
查询应用的部署Pod: `kubectl get pod -n og --show-labels`

部署的pod如下,其中og-test2-pod-172x16x0x4  为primary，：
```bash
# 本地集群所在k8s查询POD信息如下
$ kubectl get pod -n og --show-labels 
NAME                           READY   STATUS    RESTARTS   AGE     LABELS
og-test2-pod-172x16x0x4        2/2     Running   0          22m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
# 同城集群所在k8s查询POD信息如下
kubectl get pod -n og --show-labels 
NAME                           READY   STATUS    RESTARTS   AGE     LABELS
og-test2-pod-172x16x0x3   2/2     Running   0          8m52s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=standby
```
验证部署，通过本地集群容器内的 client 去连接 OG:
```bash 
$  kubectl exec -ti -n og og-test2-pod-172x16x0x4  -c og  -- gsql -d postgres -p 5432 -c "SHOW server_version;"                   
 server_version 
----------------
 9.2.4
(1 row)

```
通过容器内的数据库服务控制工具`gs_ctl`去连接，查看og集群的详细信息:

```bash
$ kubectl exec -ti -n og og-test2-pod-172x16x0x4  -c og  -- gs_ctl query -D gaussdata/openGauss/db1/
[2022-06-07 11:02:31.953][97544][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 25793
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/4002E30
        sender_write_location          : 0/4002E30
        sender_flush_location          : 0/4002E30
        sender_replay_location         : 0/4002E30
        receiver_received_location     : 0/4002E30
        receiver_write_location        : 0/4002E30
        receiver_flush_location        : 0/4002E30
        receiver_replay_location       : 0/4002E30
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.4:5434-->172.16.0.3:41514

 Receiver info:      
No information 
```
结果如上代表同城双集群部署成功.


# 同城切换
同城切换功能，只需要分别修改本地集群和同城集群CR的localrole即可

示例: 现有一个支持同城的集群test2，初始primary节点在k8s1集群上，spec内容如下:
```yaml
...
Spec:
  Archivelogpath:  /data/k8slocalogarchive
  Backuppath:      /data/k8slocalxbk
  Cpu:             500m
  Dbport:          5432
  Filebeatconfig:  opengauss-filebeat-config
  Image:           opengauss.sig/opengauss/container:2.0.0.v7
  Iplist:
    Ip:        172.16.0.4
    Nodename:  node1
  Localrole:   primary
  Memory:      2Gi
  Readport:    30003
  Storage:     5Gi
  Writeport:   30004
  Remoteiplist:
    172.16.0.3
...
```
standby节点在k8s2集群上，spec内容如下:
```yaml
...
Spec:
  Archivelogpath:  /data/k8slocalogarchive
  Backuppath:      /data/k8slocalxbk
  Cpu:             500m
  Dbport:          5432
  Filebeatconfig:  opengauss-filebeat-config
  Image:           opengauss.sig/opengauss/container:2.0.0.v7
  Iplist:
    Ip:        172.16.0.3
    Nodename:  node1
  Localrole:   standby
  Memory:      2Gi
  Readport:    30003
  Storage:     5Gi
  Writeport:   30004
  Remoteiplist:
    172.16.0.4
...
```
修改本地集群CR的localrole为standby，待本地集群的primary pod labels标记为standby后，修改同城集群CR的localrole为primary，防止出现双主
切换过程中，本地集群的cr状态由ready变为update，之后可能为变为recovering，待切换完成后，会变为ready，本地集群的原primary pod的label由primary变为standby。本地集群的cr状态和pod状态变化如下：
```bash
# 原本地集群的cr变换如下
kubectl get opengaussclusters.opengauss.sig -n og 
test2        primary   500m   2Gi      30003       30004        5432      ready  
test2        standby   500m   2Gi      30003       30004        5432      ready   105m
test2        standby   500m   2Gi      30003       30004        5432      updating   10
test2        standby   500m   2Gi      30003       30004        5432      ready        105m
test2        standby   500m   2Gi      30003       30004        5432      ready        106m
test2        standby   500m   2Gi      30003       30004        5432      recovering   106m
...     
test2        standby   500m   2Gi      30003       30004        5432      ready        107m
# 原本地集群的pod变换如下
$ kubectl get pod -n og --show-labels -w 
og-test2-pod-172x16x0x4        2/2     Running   0          103m    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
og-test2-pod-172x16x0x4        2/2     Running   0          105m    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2
og-test2-pod-172x16x0x4        2/2     Running   0          105m    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=standby
```
切换过程中，同城集群的cr状态由ready变为update，之后可能为变为recovering，待切换完成后，会变为ready，同城集群的会选择一个为主（本示例本地/同城都仅有一个节点）。同城集群的cr状态和pod状态变化如下：

```bash
# 原同城集群的cr变换如下
$ kubectl get opengaussclusters.opengauss.sig -n og -w
test2   standby   500m   2Gi      30003       30004        5432      ready      91m
test2   standby   500m   2Gi      30003       30004        5432      recovering   91m
...
test2   primary   500m   2Gi      30003       30004        5432      updating     93m
test2   primary   500m   2Gi      30003       30004        5432      updating     93m
test2   primary   500m   2Gi      30003       30004        5432      updating     93m
test2   primary   500m   2Gi      30003       30004        5432      ready        93m
# 原同城集群的pod变换如下
$ kubectl get pod -n og --show-labels -w 
og-test2-pod-172x16x0x3   2/2     Running   0          92m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=standby
og-test2-pod-172x16x0x3   2/2     Running   0          93m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
```

验证灾备切换是否成功，通过容器内的数据库服务控制工具 gs_ctl去连接 查看原同城集群的详细信息:
```bash
$ kubectl exec -ti -n og og-test2-pod-172x16x0x3   -c og  -- gs_ctl query -D gaussdata/openGauss/db1/
[2022-06-07 12:08:06.993][46173][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 18085
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/4007578
        sender_write_location          : 0/4007578
        sender_flush_location          : 0/4007578
        sender_replay_location         : 0/4007578
        receiver_received_location     : 0/4007578
        receiver_write_location        : 0/4007578
        receiver_flush_location        : 0/4007578
        receiver_replay_location       : 0/4007460
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.3:5434-->172.16.0.4:49288

 Receiver info:      
No information 
```
结果如上代表同城切换成功.