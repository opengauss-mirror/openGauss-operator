# operator故障恢复案例
使用make deploy 部署operator时，采用的方式为Deployment部署，可以设置多副本数（例如设置为3），从而实现operator高可用，当一个opertor的pod故障后，其他opertor可以继续正常工作，同一时间，只有一个operator正常工作。operator默认部署在 opengauss-operator-system命名空间下。 
设置operator的副本数，在manager.yaml中设置
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: system
  labels:
    control-plane: controller-manager
spec:
  selector:
    matchLabels:
      control-plane: controller-manager
  replicas: 1 #此处可以设置部署的pod个数，建议设置为3
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
      - command:
        - /usr/local/bin/opengauss-operator
        args:
        - --leader-elect
        image: controller:latest
        name: manager
        securityContext:
          allowPrivilegeEscalation: false
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 100m
            memory: 300Mi
          requests:
            cpu: 100m
            memory: 20Mi
      serviceAccountName: controller-manager
      terminationGracePeriodSeconds: 10
```
#### 多副本验证
按照上述方式修改manager.yaml中的replicas,修改为3，即部署3副本的operator
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: system
  labels:
    control-plane: controller-manager
spec:
  selector:
    matchLabels:
      control-plane: controller-manager
  replicas: 3
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      securityContext:
        runAsNonRoot: true
···
```
部署成功后，可以看到operator命名空间下有三个pod
```
$ kubectl get deployments.apps  -n opengauss-operator-system               
NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
opengauss-operator-controller-manager   3/3     3            3           32s
$ kubectl get pod  -n opengauss-operator-system              
NAME                                                     READY   STATUS    RESTARTS   AGE
opengauss-operator-controller-manager-5d7c58c7d5-2hk5l   1/1     Running   0          34s
opengauss-operator-controller-manager-5d7c58c7d5-vx7rn   1/1     Running   0          34s
opengauss-operator-controller-manager-5d7c58c7d5-z79ns   1/1     Running   0          34s
```
Deployment方式部署的operator，同一时间只有一个pod在工作，当其中工作的operator pod故障后，其余的pod会竞争锁，获得锁的pod会继续工作，如下查看operator的pod日志
```bash
$ kubectl logs -f -n opengauss-operator-system  opengauss-operator-controller-manager-5d7c58c7d5-2hk5l 
I0606 09:36:09.354287       1 request.go:655] Throttling request took 1.003551618s, request: GET:https://10.96.0.1:443/apis/apiextensions.k8s.io/v1?timeout=32s
2022-06-06T09:36:10.547Z        INFO    controller-runtime.metrics      metrics server is starting to listen    {"addr": "127.0.0.1:8080"}
2022-06-06T09:36:10.549Z        INFO    setup   starting manager
I0606 09:36:10.549741       1 leaderelection.go:243] attempting to acquire leader lease opengauss-operator-system/9e66c0cd.sig...
2022-06-06T09:36:10.550Z        INFO    controller-runtime.manager      starting metrics server {"path": "/metrics"}


$ kubectl logs -f -n opengauss-operator-system  opengauss-operator-controller-manager-5d7c58c7d5-z79ns 
I0606 09:36:08.687378       1 request.go:655] Throttling request took 1.042390986s, request: GET:https://10.96.0.1:443/apis/redis.shindata.com/v1?timeout=32s
2022-06-06T09:36:09.946Z        INFO    controller-runtime.metrics      metrics server is starting to listen    {"addr": "127.0.0.1:8080"}
2022-06-06T09:36:10.036Z        INFO    setup   starting manager
I0606 09:36:10.038941       1 leaderelection.go:243] attempting to acquire leader lease opengauss-operator-system/9e66c0cd.sig...
2022-06-06T09:36:10.039Z        INFO    controller-runtime.manager      starting metrics server {"path": "/metrics"}

$ kubectl logs -f -n opengauss-operator-system  opengauss-operator-controller-manager-5d7c58c7d5-vx7rn 
···
2022-06-06T09:40:34.502Z        INFO    controllers.OpenGaussCluster    [og:ogtest]开始处理集群
2022-06-06T09:40:34.503Z        DEBUG   controllers.OpenGaussCluster    [og:og-ogtest-pod-172x16x0x2]执行命令：bash /gauss/files/K8SChkRepl.sh
2022-06-06T09:40:35.480Z        DEBUG   controllers.OpenGaussCluster    [og:ogtest]位于Pod og-ogtest-pod-172x16x0x2上的数据库状态：[Local Role: Standby, Process exist: true, Connection available: true, DB state normal: true, Maintenance: false, Backup status: no data, Restore status: no data, Static Connections: 1, Detail Information: Normal, ]
2022-06-06T09:40:35.481Z        DEBUG   controllers.OpenGaussCluster    [og:og-ogtest-pod-172x16x0x5]执行命令：bash /gauss/files/K8SChkRepl.sh
2022-06-06T09:40:36.540Z        DEBUG   controllers.OpenGaussCluster    [og:ogtest]位于Pod og-ogtest-pod-172x16x0x5上的数据库状态：[Local Role: Primary, Process exist: true, Connection available: true, DB state normal: true, Maintenance: false, Backup status: no data, Restore status: no data, Static Connections: 1, Detail Information: Normal, ]
2022-06-06T09:40:36.541Z        DEBUG   controllers.OpenGaussCluster    [og:ogtest]集群状态正常
2022-06-06T09:40:36.541Z        DEBUG   controllers.OpenGaussCluster    [og:og-ogtest-pod-172x16x0x5]执行命令：bash /gauss/files/K8SChkRepl.sh
2022-06-06T09:40:37.632Z        DEBUG   controllers.OpenGaussCluster    [og:ogtest]位于Pod og-ogtest-pod-172x16x0x5上的数据库状态：[Local Role: Primary, Process exist: true, Connection available: true, DB state normal: true, Maintenance: false, Backup status: no data, Restore status: no data, Static Connections: 1, Detail Information: Normal, ]
2022-06-06T09:40:37.632Z        DEBUG   controllers.OpenGaussCluster    [og:og-ogtest-pod-172x16x0x5]执行命令：gs_ctl query -D /gaussdata/openGauss/db1
2022-06-06T09:40:38.289Z        INFO    controllers.OpenGaussCluster    [og:ogtest]集群处理完成
```

如上，可以看到，只有opengauss-operator-controller-manager-5d7c58c7d5-vx7rn pod在工作。