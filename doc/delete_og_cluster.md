# 删除openGauss集群
删除openGauss集群，只需要执行k8s命令删除cr即可。但是需要注意的是，**删除openGauss集群后，CR的pvc仍然存在**
> kubectl delete opengaussclusters.opengauss.sig -n \<namespace name>  \<cr name> 

og命名空间下部署了一个一主一从的og集群 ogtest
```bash
$ kubectl get opengaussclusters.opengauss.sig  -n og 
NAME     ROLE      CPU    MEMORY   READ PORT   WRITE PORT   DB PORT   STATE   AGE
ogtest   primary   500m   2Gi      30001       30002        5432      ready   29m
```

执行k8s删除命令，删除cr
```bash
$ kubectl delete opengaussclusters.opengauss.sig  -n og ogtest 
opengausscluster.opengauss.sig "ogtest" deleted
[root@xxx og-sig]# $ kubectl get opengaussclusters.opengauss.sig  -n og            
No resources found in og namespace.
```

删除cr资源后，其PVC仍然保留，以防止需要恢复数据
查看og集群的pvc
>kubectl get pvc -n  \<namespace name> |grep \<cr name> 
```bash
$ kubectl get pvc -n og |grep ogtest
NAME                                STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
og-ogtest-pod-172x16x0x2-data-pvc   Bound    pvc-6a0b679b-cf03-4cc3-ad3b-605afd7e1911   5Gi        RWO            topolvm-provisioner   3d15h
og-ogtest-pod-172x16x0x2-log-pvc    Bound    pvc-474acf02-43df-44e8-80cf-fba1b9d5f82b   1Gi        RWO            topolvm-provisioner   3d15h
og-ogtest-pod-172x16x0x3-data-pvc   Bound    pvc-e135b0ee-2f54-474f-a24a-b0a8425b12f7   5Gi        RWO            topolvm-provisioner   38m
og-ogtest-pod-172x16x0x3-log-pvc    Bound    pvc-93aa3f9d-5fe8-4309-946f-186fafc011c4   1Gi        RWO            topolvm-provisioner   38m
```

确认数据不需要保存时，直接删除PVC资源即可，避免资源的浪费

>kubectl delete pvc \<pvc name> -n \<namespace name>

删除pvc资源如下
```bash
$ kubectl delete pvc og-ogtest-pod-172x16x0x2-data-pvc og-ogtest-pod-172x16x0x2-log-pvc og-ogtest-pod-172x16x0x3-data-pvc og-ogtest-pod-172x16x0x3-log-pvc  -n og
persistentvolumeclaim "og-ogtest-pod-172x16x0x2-data-pvc" deleted
persistentvolumeclaim "og-ogtest-pod-172x16x0x2-log-pvc" deleted
persistentvolumeclaim "og-ogtest-pod-172x16x0x3-data-pvc" deleted
persistentvolumeclaim "og-ogtest-pod-172x16x0x3-log-pvc" deleted
```