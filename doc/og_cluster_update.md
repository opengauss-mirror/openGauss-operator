# openGauss集群资源升级
opertor支持og集群的资源升级，即修改已有集群的内存，CPU,带宽，存储容量等大小。需要特别注意的是： 
- **存储容量仅支持扩容，不支持缩容**
- **如果资源调整涉及cpu和内存、带宽资源调整，会重启pod。同时如果为多节点部署，会发生*主从切换***
  
示例：k8s集群og命名空间下有一个一主一从的og集群`ogtest`，初始CPU：600m，Memory：3Gi，存储（Storage）：6Gi
```bash
$ kubectl describe opengaussclusters.opengauss.sig -n og ogtest 
...
Spec:
  Archivelogpath:  /data/k8slocalogarchive
  Backuppath:      /data/k8slocalxbk
  Cpu:             600m
  Dbport:          5432
  Filebeatconfig:  opengauss-filebeat-config
  Image:           opengauss.sig/opengauss/container:2.0.0.v7
  Iplist:
    Ip:        172.16.0.2
    Nodename:  node1
    Ip:        172.16.0.5
    Nodename:  node2
  Localrole:   primary
  Memory:      3Gi
  Readport:    30001
  Storage:     6Gi
...

#查看初始pvc大小
kubectl get pvc -n og  |grep ogtest |grep data
og-ogtest-pod-172x16x0x2-data-pvc   Bound    pvc-80c7693c-ed77-42a9-b39f-79e46f8e02ec   8Gi        RWO            topolvm-provisioner   3h18m
og-ogtest-pod-172x16x0x5-data-pvc   Bound    pvc-5b136c05-2b00-45b6-babf-6ba126406938   6Gi        RWO            topolvm-provisioner   6m6s
```
同时修改多个属性，例如cpu改为600m，memory：3Gi,Storage:6Gi
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
  storage: 8Gi
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
重新apply后，CR state由ready变为updating，开始资源调整操作，操作完成后会再次变为ready,如下：
```bash
#资源调整过程，cr state变化如下
$ kubectl get opengaussclusters.opengauss.sig  -n og -w
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogtest   primary   600m   3Gi      30001       30002        5432      ready   3h46m
ogtest   primary   500m   2Gi      30001       30002        5432      ready   3h46m
ogtest   primary   500m   2Gi      30001       30002        5432      ready   3h46m
ogtest   primary   500m   2Gi      30001       30002        5432      ready   3h46m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   3h46m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   3h46m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   3h46m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   3h46m
ogtest   primary   500m   2Gi      30001       30002        5432      updating   3h50m
ogtest   primary   500m   2Gi      30001       30002        5432      ready      3h50m
ogtest   primary   500m   2Gi      30001       30002        5432      ready      3h51m
ogtest   primary   500m   2Gi      30001       30002        5432      ready      3h51m
ogtest   primary   500m   2Gi      30001       30002        5432      recovering   3h51m
ogtest   primary   500m   2Gi      30001       30002        5432      recovering   3h51m
ogtest   primary   500m   2Gi      30001       30002        5432      recovering   3h51m
ogtest   primary   500m   2Gi      30001       30002        5432      ready        3h51m
#资源调整涉及了cpu、内存和存储容量，pod会重启，且会发生主从切换
$ kubectl get pod -n og --show-labels -w
NAME                       READY   STATUS    RESTARTS   AGE     LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          11m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x5   2/2     Running   0          5m11s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   2/2     Terminating   0          5m36s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   0/2     Terminating   0          6m8s    app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   0/2     Terminating   0          6m22s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   0/2     Terminating   0          6m22s   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   0/2     Pending       0          0s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Pending       0          0s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Init:0/1      0          0s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Init:0/1      0          6s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     Init:0/1      0          7s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   0/2     PodInitializing   0          8s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   1/2     Running           0          9s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   1/2     Running           0          53s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   2/2     Running           0          13m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x5   2/2     Running           0          70s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   2/2     Running           0          14m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   2/2     Running           0          99s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
og-ogtest-pod-172x16x0x2   2/2     Terminating       0          14m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   0/2     Terminating       0          14m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   0/2     Terminating       0          14m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   0/2     Terminating       0          14m     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   0/2     Pending           0          0s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   0/2     Pending           0          0s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   0/2     Init:0/1          0          0s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   0/2     Init:0/1          0          9s      app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   0/2     Init:0/1          0          10s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   0/2     PodInitializing   0          11s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   1/2     Running           0          12s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest
og-ogtest-pod-172x16x0x2   1/2     Running           0          53s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x2   2/2     Running           0          74s     app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
```
验证扩容是否成功
查看ogtest集群的data pvc大小
```bash
$ kubectl get pvc -n og |grep ogtest |grep data
og-ogtest-pod-172x16x0x2-data-pvc   Bound    pvc-80c7693c-ed77-42a9-b39f-79e46f8e02ec   8Gi        RWO            topolvm-provisioner   3h32m
og-ogtest-pod-172x16x0x5-data-pvc   Bound    pvc-5b136c05-2b00-45b6-babf-6ba126406938   8Gi        RWO            topolvm-provisioner   20m
```
查看ogtest集群的pod信息,primary由原来的og-ogtest-pod-172x16x0x2切换到了og-ogtest-pod-172x16x0x5 
```bash
kubectl get pod -n og --show-labels 
NAME                       READY   STATUS    RESTARTS   AGE   LABELS
og-ogtest-pod-172x16x0x2   2/2     Running   0          13m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=standby
og-ogtest-pod-172x16x0x5   2/2     Running   0          15m   app.kubernetes.io/app=opengauss,app.kubernetes.io/name=ogtest,opengauss.role=primary
```
进入容器内执行gs_ctl 命令
```
#进入primary og-ogtest-pod-172x16x0x5 的pod中，执行gs_ctl查看集群详情
$ kubectl exec -ti -n og og-ogtest-pod-172x16x0x5 -c og -- gs_ctl query -D gaussdata/openGauss/db1
[2022-05-31 18:12:05.862][28093][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 3330
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/6001720
        sender_write_location          : 0/6001720
        sender_flush_location          : 0/6001720
        sender_replay_location         : 0/6001720
        receiver_received_location     : 0/6001720
        receiver_write_location        : 0/6001720
        receiver_flush_location        : 0/6001720
        receiver_replay_location       : 0/6001720
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 172.16.0.5:5434-->172.16.0.2:51630

 Receiver info:      
No information 
```
结果如上，代表资源升级操作成功。