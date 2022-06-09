# openGauss集群故障案例
operator会监控集群中 og集群的pod，当监测到pod故障后，会尝试将其重新拉起
## 单节点pod故障
示例
og命名空间下有一个单节点的og应用test2，状态正常
```bash
$ kubectl get opengaussclusters.opengauss.sig  -n og
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
test2    primary   500m   2Gi      30003       30004        5432      ready   19h
```
应用test的spec内容如下：
```bash
$ kubectl describe opengaussclusters.opengauss.sig  -n og  test2
...
Spec:
  Archivelogpath:  /data/k8slocalogarchive
  Backuppath:      /data/k8slocalxbk  
  Cpu:                          500m
  Dbport:                       5432
  Filebeatconfig:               opengauss-filebeat-config
  Image:                        opengauss.sig/opengauss/container:2.0.0.v7
  Iplist:
    Ip:        172.16.0.4
    Nodename:  node1
  Localrole:   primary
  Memory:      2Gi
  Readport:    30003
...
```
初始 test2应用下的pod如下,且在postgres下有一张表，表里有32条数据
```bash
$ kubectl  get pod -n og --show-labels 
og-test2-pod-172x16x0x4    2/2     Running   0          19h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary

# 查看表中初始的数据
$ kubectl exec -it -n og og-test2-pod-172x16x0x4 -c og -- gsql -d postgres -p 5432 -c "select count(*) from test_1"
 count 
-------
    32
(1 row)
```

删除test2的pod
```bash
$ kubectl delete pod -n og og-test2-pod-172x16x0x4 
pod "og-test2-pod-172x16x0x4" deleted
```
operator监控到test2下的pod被删除后，会尝试恢复它，即CR状态变为 recovering
删除pod到pod恢复过程中，CR状态变化如下：
```bash
kubectl get opengaussclusters.opengauss.sig  -n og -w 
test2    primary   500m   2Gi      30003       30004        5432      ready   19h
test2    primary   500m   2Gi      30003       30004        5432      recovering   19h
test2    primary   500m   2Gi      30003       30004        5432      recovering   19h
test2    primary   500m   2Gi      30003       30004        5432      recovering   19h
test2    primary   500m   2Gi      30003       30004        5432      recovering   19h
test2    primary   500m   2Gi      30003       30004        5432      ready   19h
```
test2应用的pod变化如下：
```bash
kubectl  get pod -n og --show-labels -w |grep test2
og-test2-pod-172x16x0x4    2/2     Running   0          19h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
og-test2-pod-172x16x0x4    2/2     Terminating   0          19h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
og-test2-pod-172x16x0x4    0/2     Terminating   0          19h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
og-test2-pod-172x16x0x4    0/2     Terminating   0          19h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
og-test2-pod-172x16x0x4    0/2     Terminating   0          19h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
og-test2-pod-172x16x0x4    0/2     Pending       0          0s    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2
og-test2-pod-172x16x0x4    0/2     Pending       0          0s    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2
og-test2-pod-172x16x0x4    0/2     Init:0/1      0          0s    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2
og-test2-pod-172x16x0x4    0/2     Init:0/1      0          10s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2
og-test2-pod-172x16x0x4    0/2     Init:0/1      0          11s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2
og-test2-pod-172x16x0x4    0/2     PodInitializing   0          12s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2
og-test2-pod-172x16x0x4    1/2     Running           0          13s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2
og-test2-pod-172x16x0x4    1/2     Running           0          35s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
og-test2-pod-172x16x0x4    2/2     Running           0          73s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
og-test2-pod-172x16x0x4    2/2     Running           0          5m    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
og-test2-pod-172x16x0x4    2/2     Running           0          5m15s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=test2,opengauss.role=primary
```
验证恢复后，db的状态，及数据是否丢失
```bash
#查看db状态
$ kubectl exec -it -n og og-test2-pod-172x16x0x4 -c og -- gs_ctl query -D gaussdata/openGauss/db1/
[2022-06-02 10:23:19.570][54530][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
No information 
 Receiver info:      
No information 
# 查看test_1表中的数据
$ kubectl exec -it -n og og-test2-pod-172x16x0x4 -c og -- gsql -d postgres -p 5432 -c "select count(*) from test_1"
 count 
-------
    32
(1 row)
```
如上，验证通过，pod down后，operator会尝试恢复它，且过程中数据不会丢失.
## 一主一从Primary pod故障
示例
og命名空间下有一个单节点的og应用ogtest，状态正常
```bash
$  kubectl get opengaussclusters.opengauss.sig  -n og 
ogtest   primary   500m   2Gi      30001       30002        5432      ready   2d4h
```
应用test的spec内容如下：
```bash
$ kubectl describe opengaussclusters.opengauss.sig  -n og  test2
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
...
```

初始 ogtest集群下的pod如下,primary的Pod 为og-ogtest-pod-172x16x0x5,且在postgres下有一张表，表里有32条数据
```
$ kubectl get pod -n og --show-labels 
og-ogtest-pod-172x16x0x2   2/2     Running   0          2d1h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   2/2     Running   0          2d1h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary

# 查看表中初始的数据
$  kubectl exec -it -n og og-ogtest-pod-172x16x0x5 -c og -- gsql -d postgres -p 5432 -c "select *  from test_1" 
 id |   name   
----+----------
 1  | zhangsan
 2  | lisi
(2 rows)

# 查看db状态
kubectl exec -it -n og og-ogtest-pod-172x16x0x5 -c og -- gs_ctl query -D gaussdata/openGauss/db1/
[2022-06-02 19:11:14.678][12621][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 18133
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/77FA8D8
        sender_write_location          : 0/77FA8D8
        sender_flush_location          : 0/77FA8D8
        sender_replay_location         : 0/77FA8D8
        receiver_received_location     : 0/77FA8D8
        receiver_write_location        : 0/77FA8D8
        receiver_flush_location        : 0/77FA8D8
        receiver_replay_location       : 0/77FA8D8
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.5:5434-->172.16.0.2:44514

 Receiver info:      
No information 
```

删除角色为primary的pod
```
$ kubectl delete pod -n og og-ogtest-pod-172x16x0x5
pod "og-ogtest-pod-172x16x0x5" deleted
```
operator监控到ogtest下的primary pod被删除后，会尝试恢复它，即CR状态变为 recovering
删除pod到pod恢复过程中，CR状态变化如下：
```bash
 kubectl get opengaussclusters.opengauss.sig -n og -w |grep ogtest
ogtest   primary   500m   2Gi      30001       30002        5432      ready   2d5h
ogtest   primary   500m   2Gi      30001       30002        5432      ready   2d5h
ogtest   primary   500m   2Gi      30001       30002        5432      ready   2d5h
ogtest   primary   500m   2Gi      30001       30002        5432      recovering   2d5h
ogtest   primary   500m   2Gi      30001       30002        5432      recovering   2d5h
ogtest   primary   500m   2Gi      30001       30002        5432      ready        2d5h
```
test2应用的pod变化如下：
```bash
kubectl get pod -n og --show-labels -w |grep ogtest 
og-ogtest-pod-172x16x0x2   2/2     Running   0          2d1h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   2/2     Running   0          2d1h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x5   2/2     Terminating   0          2d1h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x5   0/2     Terminating   0          2d1h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x5   0/2     Terminating   0          2d1h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x5   0/2     Terminating   0          2d1h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x5   0/2     Pending       0          0s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Pending       0          0s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Init:0/1      0          0s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Init:0/1      0          4s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Init:0/1      0          5s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     PodInitializing   0          6s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   1/2     Running           0          8s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   1/2     Running           0          33s    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x5   2/2     Running           0          69s    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
```
验证恢复后，db的状态，及数据是否丢失
```bash
# 查看db状态
$ kubectl exec -it -n og og-ogtest-pod-172x16x0x5 -c og -- gs_ctl query -D gaussdata/openGauss/db1/
[2022-06-02 19:16:29.680][5421][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 755
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/77FAF68
        sender_write_location          : 0/77FAF68
        sender_flush_location          : 0/77FAF68
        sender_replay_location         : 0/77FAF68
        receiver_received_location     : 0/77FAF68
        receiver_write_location        : 0/77FAF68
        receiver_flush_location        : 0/77FAF68
        receiver_replay_location       : 0/77FAF68
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.5:5434-->172.16.0.2:43856

 Receiver info:      
No information 

# 查看表中初始的数据
$ kubectl exec -it -n og og-ogtest-pod-172x16x0x5 -c og -- gsql -d postgres -p 5432 -c "select *  from test_1" 
 id |   name   
----+----------
 1  | zhangsan
 2  | lisi
(2 rows)

```
查看恢复后ogtest集群下的pod
```bash
$ kubectl get pod -n og --show-labels -w |grep ogtest 
og-ogtest-pod-172x16x0x2   2/2     Running   0          2d1h    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   2/2     Running   0          4m58s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
```
如上，验证通过，primary pod 挂掉后，operator会尝试将其修复，且保持角色不变，仍然为主，数据不会丢失.
## 一主一从Standby pod故障
示例
og命名空间下有一个单节点的og应用ogtest，状态正常
```bash
$  kubectl get opengaussclusters.opengauss.sig  -n og |grep ogtest
ogtest   primary   500m   2Gi      30001       30002        5432      ready   6d
```
应用test的spec内容如下：
```bash
$ kubectl describe opengaussclusters.opengauss.sig  -n og  ogtest
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
...
```

初始 ogtest集群下的pod如下,primary的Pod 为og-ogtest-pod-172x16x0x5,且在postgres下有一张表，表里有32条数据
```bash
$ kubectl get pod -n og --show-labels  |grep ogtest
og-ogtest-pod-172x16x0x2   2/2     Running   0          5d20h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   2/2     Running   0          3d19h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary

# 查看表中初始的数据
$  kubectl exec -it -n og og-ogtest-pod-172x16x0x5 -c og -- gsql -d postgres -p 5432 -c "select *  from test_1" 
 id |   name   
----+----------
 1  | zhangsan
 2  | lisi
(2 rows)

# 查看db状态
$  kubectl exec -it -n og og-ogtest-pod-172x16x0x5 -c og -- gs_ctl query -D gaussdata/openGauss/db1/
[2022-06-06 14:18:05.369][91656][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 755
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/C418240
        sender_write_location          : 0/C418240
        sender_flush_location          : 0/C418240
        sender_replay_location         : 0/C418240
        receiver_received_location     : 0/C418240
        receiver_write_location        : 0/C418240
        receiver_flush_location        : 0/C418240
        receiver_replay_location       : 0/C418240
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.5:5434-->172.16.0.2:43856

 Receiver info:      
No information 
```

删除角色为standby的pod
```bash
$  kubectl delete pod -n og og-ogtest-pod-172x16x0x2 
pod "og-ogtest-pod-172x16x0x2" deleted
```
operator监控到ogtest下的primary pod被删除后，会尝试恢复它，即CR状态变为 recovering
删除pod到pod恢复过程中，CR状态变化如下：
```bash
$ kubectl get opengaussclusters.opengauss.sig  -n og -w |grep ogtest
ogtest   primary   500m   2Gi      30001       30002        5432      ready   6d
ogtest   primary   500m   2Gi      30001       30002        5432      ready   6d
ogtest   primary   500m   2Gi      30001       30002        5432      recovering   6d
ogtest   primary   500m   2Gi      30001       30002        5432      recovering   6d
ogtest   primary   500m   2Gi      30001       30002        5432      ready        6d
```
test2应用的pod变化如下：
```bash
kubectl get pod -n og --show-labels -w  |grep ogtest                                                            
og-ogtest-pod-172x16x0x2   2/2     Running   0          5d20h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   2/2     Running   0          3d19h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x2   2/2     Terminating   0          5d20h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   0/2     Terminating   0          5d20h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   0/2     Terminating   0          5d20h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   0/2     Terminating   0          5d20h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   0/2     Pending       0          0s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   0/2     Pending       0          0s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   0/2     Init:0/1      0          0s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   0/2     Init:0/1      0          9s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   0/2     PodInitializing   0          10s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   1/2     Running           0          11s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   1/2     Running           0          28s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   2/2     Running           0          72s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby

```
删除过程中，主节点仍正常工作，可正常插入数据
```bash
kubectl exec -ti -n og og-ogtest-pod-172x16x0x5 bash
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
Defaulting container name to og.
Use 'kubectl describe pod/og-ogtest-pod-172x16x0x5 -n og' to see all of the containers in this pod.
[omm@og-ogtest-pod-172x16x0x5 /]$ gsql -d postgres -p 5432 -r
gsql ((openGauss 2.0.0 build 01c5f150) compiled at 2021-11-13 18:58:44 commit 0 last mr  )
Non-SSL connection (SSL connection is recommended when requiring high-security)
Type "help" for help.

postgres=# insert into test_1 values('22','20220606');
INSERT 0 1
```
验证恢复后，db的状态，及数据是否丢失
```bash
# 查看db状态
$ kubectl exec -it -n og og-ogtest-pod-172x16x0x5 -c og -- gs_ctl query -D gaussdata/openGauss/db1/
[2022-06-06 14:29:14.581][111483][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 98167
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/C418F40
        sender_write_location          : 0/C418F40
        sender_flush_location          : 0/C418F40
        sender_replay_location         : 0/C418F40
        receiver_received_location     : 0/C418F40
        receiver_write_location        : 0/C418F40
        receiver_flush_location        : 0/C418F40
        receiver_replay_location       : 0/C418F40
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.5:5434-->172.16.0.2:33450

 Receiver info:      
No information  

# 查看表中的数据，可以查到新增的数据
$  kubectl exec -it -n og og-ogtest-pod-172x16x0x5 -c og -- gsql -d postgres -p 5432 -c "select *  from test_1" 
 id |   name   
----+----------
 1  | zhangsan
 2  | lisi
 22 | 20220606
(3 rows)

```
查看恢复后ogtest集群下的pod
```bash
$  kubectl get pod -n og --show-labels -w  |grep ogtest
og-ogtest-pod-172x16x0x2   2/2     Running   0          14m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   2/2     Running   0          3d19h   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
```
standby pod 挂掉后，operator会尝试将其修复，且保持角色不变，仍然为从，从挂掉的过程中，主仍可以正常工作，可以正常写数据.
# 存储故障
## 可分配存储不足（VG不足）
当og集群指定部署的Node 可用VG空间不足时，会一直不成功，卡在creating状态
示例
集群部署的node1下可用VG为557.99G，如下:
```bash
$  vgs
  VG     #PV #LV #SN Attr   VSize VFree  
  centos   4  86   0 wz--n- 1.85t 557.99g
```
现在该节点部署一个单节点og集群storage-test ，存储容量设置为1Ti，如下:
```bash
$ cat <<EOF |  kubectl apply -f -
apiVersion: opengauss.sig/v1
kind: OpenGaussCluster
metadata:
  name: storage-test
  namespace: test
spec:
  cpu: 500m
  dbport: 5432
  image: opengauss.sig/opengauss/container:2.0.0.v7
  memory: 2Gi
  storage: 1Ti
  storageclass: topolvm-provisioner
  readport: 30010
  writeport: 30011
  iplist:
  - ip: 172.16.0.6
    nodename: k8snew70
EOF
```
查看og集群状态，会一直处于creating状态
```bash
$ kubectl get opengaussclusters.opengauss.sig -n test -w
NAME           ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE      AGE
storage-test   primary   500m   2Gi      30010       30011        5432      creating   10s
storage-test   primary   500m   2Gi      30010       30011        5432      creating   5m6s
storage-test   primary   500m   2Gi      30010       30011        5432      creating   5m36s
storage-test   primary   500m   2Gi      30010       30011        5432      creating   9m14s
```
使用`kubectl describe`命令查看 集群下pod的详情,提示out of VG free space
```bash
 kubectl describe pod -n test og-storage-test-pod-172x16x0x6 
Name:           og-storage-test-pod-172x16x0x6
Namespace:      test
Priority:       0
Node:           <none>
Labels:         app.kubernetes.io/app=opengauss
                app.kubernetes.io/name=storage-test
Annotations:    capacity.topolvm.cybozu.com/00default: 
...
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  45s   default-scheduler  0/3 nodes are available: 1 Insufficient cpu, 1 node(s) didn't match node selector, 1 out of VG free space.
  Warning  FailedScheduling  45s   default-scheduler  0/3 nodes are available: 1 Insufficient cpu, 1 node(s) didn't match node selector, 1 out of VG free space.
```
如上场景只能通过修改og集群的storage大小或者给对应机器增加存储（扩容VG容量）。
## data pvc存储空间不足 
opengauss集群部署成功后，会创建两个pvc：data pvc和log pvc
data pvc用于存储数据，log pvc用户存储日志
当data pvc存储容量满后，会导致数据无法写入，operator无法干预，需要人为主动修改og集群CR 对应的storage或清理无用数据
示例
test命名空间下部署了一个单节点 test_stroage,其spec信息如下
```bash
$ kubectl describe opengaussclusters.opengauss.sig  -n test
...
Spec:
  Archivelogpath:  /data/k8slocalogarchive
  Backuppath:      /data/k8slocalxbk
  Cpu:             500m
  Dbport:          5432
  Filebeatconfig:  opengauss-filebeat-config
  Image:           opengauss.sig/opengauss/container:2.0.0.v7
  Iplist:
    Ip:        172.16.0.6
    Nodename:  node1
  Localrole:   primary
  Memory:      2Gi
  Readport:    30011
...
```
进入pod 查看其pvc的容量使用情况
data pvc对应pod的挂载路径为/gaussdata/openGauss
log pvc的对应pod的挂载路径为/gaussarch

```bash
kubectl exec -it -n test og-storage-test-pod-172x16x0x6 bash 
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
Defaulting container name to og.
Use 'kubectl describe pod/og-storage-test-pod-172x16x0x6 -n test' to see all of the containers in this pod.
[omm@og-storage-test-pod-172x16x0x6 /]$ df -h
Filesystem                                         Size  Used Avail Use% Mounted on
overlay                                            600G  204G  397G  34% /
tmpfs                                               64M     0   64M   0% /dev
tmpfs                                               25G     0   25G   0% /sys/fs/cgroup
/dev/topolvm/a347961f-e131-4377-b8e4-5f999b5e64a1  976M  960M     0 100% /gaussarch
shm                                                 64M   12K   64M   1% /dev/shm
/dev/mapper/centos-root                            600G  204G  397G  34% /etc/hosts
/dev/topolvm/96f21459-8e81-41a1-9596-0bbb16351d88  2.9G  2.9G     0 100% /gaussdata/openGauss
tmpfs                                               25G   12K   25G   1% /run/secrets/kubernetes.io/serviceaccount
tmpfs                                               25G     0   25G   0% /proc/acpi
tmpfs                                               25G     0   25G   0% /proc/scsi
tmpfs                                               25G     0   25G   0% /sys/firmware
```

如上可以看到data pvc和log pvc使用已经打到100%，此时无法继续往数据库中写，如下:
```bash
kubectl exec -it -n test og-storage-test-pod-172x16x0x6 bash 
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
Defaulting container name to og.
Use 'kubectl describe pod/og-storage-test-pod-172x16x0x6 -n test' to see all of the containers in this pod.
[omm@og-storage-test-pod-172x16x0x6 /]$ df -h
Filesystem                                         Size  Used Avail Use% Mounted on
overlay                                            600G  204G  397G  34% /
tmpfs                                               64M     0   64M   0% /dev
tmpfs                                               25G     0   25G   0% /sys/fs/cgroup
/dev/topolvm/a347961f-e131-4377-b8e4-5f999b5e64a1  976M  960M     0 100% /gaussarch
shm                                                 64M   12K   64M   1% /dev/shm
/dev/mapper/centos-root                            600G  204G  397G  34% /etc/hosts
/dev/topolvm/96f21459-8e81-41a1-9596-0bbb16351d88  2.9G  2.9G     0 100% /gaussdata/openGauss
tmpfs                                               25G   12K   25G   1% /run/secrets/kubernetes.io/serviceaccount
tmpfs                                               25G     0   25G   0% /proc/acpi
tmpfs                                               25G     0   25G   0% /proc/scsi
tmpfs                                               25G     0   25G   0% /sys/firmware
[omm@og-storage-test-pod-172x16x0x6 /]$ gsql -d postgres -p 5432 -r
gsql ((openGauss 2.0.0 build 01c5f150) compiled at 2021-11-13 18:58:44 commit 0 last mr  )
Non-SSL connection (SSL connection is recommended when requiring high-security)
Type "help" for help.

postgres=# select count(*) from test_1;
 count 
-------
 16384
(1 row)

postgres=# insert into test_1 (select * from test_1);
ERROR:  could not extend file "base/14244/16388": wrote only 4096 of 8192 bytes at block 115
HINT:  Check free disk space.
postgres=# 
```
以上情况，数据库无法写入数据，解决方式：通过扩容CR的storage或者清理db中的无用数据。