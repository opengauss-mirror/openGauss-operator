# 部署openGauss集群
## 单节点
部署一个简单的 OG 单节点应用:
```bash
cat <<EOF |  kubectl apply -f -
apiVersion: opengauss.sig/v1
kind: OpenGaussCluster
metadata:
  name: ogt
spec:
  image: opengauss-5.0.2:latest
  readport: 30001
  writeport: 30002
  localrole: primary
  storageclass: topolvm-provisioner
  iplist:
  - ip: 10.244.2.98
    nodename: node1
EOF
```
查询应用的部署状态:
`$ kubectl get opengausscluster ogt`

输出结果类似如下：
```bash
NAME   ROLE      CPU   MEMORY   READ PORT   WRITE PORT   DB PORT   STATE      AGE
ogt    primary   1     4Gi      30001       30002        5432      creating   12s
```

直到 STATE 到达 ready:
```bash
NAME   ROLE      CPU   MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogt    primary   1     4Gi      30001       30002        5432      ready   1m15s
```

验证部署，通过容器内的 client 去连接 OG:
```bash
$ kubectl exec -it og-ogt-pod-10x244x2x98 -c og  -- gsql -d postgres -p 5432 -c "SHOW server_version;"
 server_version
----------------
 9.2.4
(1 row)
```

结果如上代表安装成功.

## 1主1从
部署一个简单的 一主一从 OG HA应用:
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
  image: opengauss-5.0.2:latest
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
查询应用的部署状态:
`$ kubectl get opengaussclusters.opengauss.sig -n og ogtest`

输出结果类似如下：
```bash
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE      AGE
ogtest   primary   500m   2Gi      30001       30002        5432      creating   5m19s
```

直到 STATE 到达 ready:
```bash
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogtest   primary   500m   2Gi      30001       30002        5432      ready   6m16s
```
查询应用的部署Pod:
`$ kubectl get pod -n og --show-labels`

部署的pod如下,其中og-ogtest-pod-172x16x0x2为primary，og-ogtest-pod-172x16x0x3为standby：
```bash
NAME                       READY   STATUS    RESTARTS   AGE   LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          14m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x3   2/2     Running   0          14m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
```
验证部署，通过容器内的 client 去连接 OG:
```bash
$ kubectl exec -ti -n og og-ogtest-pod-172x16x0x2 -c og  -- gsql -d postgres -p 5432 -c "SHOW server_version;"
 server_version 
----------------
 9.2.4
(1 row)
```
通过容器内的数据库服务控制工具`gs_ctl`查看og集群的详细信息:

```bash
$ kubectl exec -ti -n og og-ogtest-pod-172x16x0x2 -c og  -- gs_ctl query -D gaussdata/openGauss/db1/
[2022-05-31 10:08:56.966][37322][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 6215
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/60013D8
        sender_write_location          : 0/60013D8
        sender_flush_location          : 0/60013D8
        sender_replay_location         : 0/60013D8
        receiver_received_location     : 0/60013D8
        receiver_write_location        : 0/60013D8
        receiver_flush_location        : 0/60013D8
        receiver_replay_location       : 0/60013D8
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.2:5434-->172.16.0.3:48160

 Receiver info:      
No information 
```
结果如上代表安装成功，且HA一主一从集群搭建成功.
## 1主2从
部署一个 一主两从 OG HA应用:
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
  image: opengauss-5.0.2:latest
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
  - ip: 172.16.0.4
    nodename: node3
EOF
```
查询应用的部署状态:
> kubectl get opengaussclusters.opengauss.sig -n \<namespace name >  \<cr name>

输出结果类似如下：
```bash
$ kubectl get opengaussclusters.opengauss.sig  -n og ogtest
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE      AGE
ogtest   primary   500m   2Gi      30001       30002        5432      creating   27s
```

直到 STATE 到达 ready:
```bash
$ kubectl get opengaussclusters.opengauss.sig  -n og
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogtest   primary   500m   2Gi      30001       30002        5432      ready   4m11s
```
查询应用的部署Pod:
>kubectl get pod -n og --show-labels

部署的pod如下,其中og-ogtest-pod-172x16x0x2为primary，og-ogtest-pod-172x16x0x3和og-ogtest-pod-172x16x0x4为standby：
```bash
$ kubectl get pod -n og --show-labels
NAME                       READY   STATUS    RESTARTS   AGE     LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          3m56s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x3   2/2     Running   0          3m56s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x4   2/2     Running   0          3m56s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
```
验证部署，通过容器内的 client 去连接 OG:
```bash
$ kubectl exec -ti -n og og-ogtest-pod-172x16x0x2 -c og  -- gsql -d postgres -p 5432 -c "SHOW server_version;"
 server_version 
----------------
 9.2.4
(1 row)

```
通过容器内的数据库服务控制工具`gs_ctl`查看og集群的详细信息:

```bash
$ kubectl exec -ti -n og og-ogtest-pod-172x16x0x2 -c og  -- gs_ctl query -D gaussdata/openGauss/db1/
[2022-05-31 11:18:36.465][9108][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 2
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 1157
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/40005A0
        sender_write_location          : 0/40005A0
        sender_flush_location          : 0/40005A0
        sender_replay_location         : 0/40005A0
        receiver_received_location     : 0/40005A0
        receiver_write_location        : 0/40005A0
        receiver_flush_location        : 0/40005A0
        receiver_replay_location       : 0/40005A0
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.2:5434-->172.16.0.3:49384

        sender_pid                     : 2182
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/40005A0
        sender_write_location          : 0/40005A0
        sender_flush_location          : 0/40005A0
        sender_replay_location         : 0/40005A0
        receiver_received_location     : 0/40005A0
        receiver_write_location        : 0/40005A0
        receiver_flush_location        : 0/40005A0
        receiver_replay_location       : 0/40005A0
        sync_percent                   : 100%
        sync_state                     : Potential
        sync_priority                  : 2
        sync_most_available            : Off
        channel                        : 172.16.0.2:5434-->172.16.0.4:37998

 Receiver info:      
No information  
```
结果如上代表安装成功，且HA一主二从集群搭建成功.
## 同城集群
同城集群需要分别部署两套og应用，具体的部署与切换操作请参考[openGauss同城集群详情](intracity_og_cluster.md)