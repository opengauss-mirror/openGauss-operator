# operator部署文档
本文档以minikube为基础，展示如何部署并使用operator。  
文档根据以下系统环境及依赖进行编写：
> CentOS 7.6  
> Minikube 1.20.0  
> Docker 20.10.16  
> openEuler 20.03 TLS  
> openGauss 3.0.0

# [Minikube](https://minikube.sigs.k8s.io)
## 1. 下载并安装minikube  
以rpm包形式下载安装minikube：
```bash
curl -LO https://github.com/kubernetes/minikube/releases/download/v1.20.0/minikube-1.20.0-0.x86_64.rpm  
rpm -Uvh minikube-1.20.0-0.x86_64.rpm  
```
或下载minikube二进制文件并放到系统路径中
```bash
curl -Lo minikube https://kubernetes.oss-cn-hangzhou.aliyuncs.com/minikube/releases/v1.20.0/minikube-linux-amd64
chmod +x minikube
mv minikube /usr/local/bin
```
## 2. 配置kubectl工具
minikube自带了kubectl命令，`minikube kubectl --`。向当前用户的配置文件中添加
```bash
echo 'alias kubectl="minikube kubectl --"' >>~/.bashrc  
source ~/.bashrc  
```
或直接下载[kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)
```bash
wget http://wenyu-software.oss-cn-hangzhou.aliyuncs.com/kubectl-v1.20.0 -o /usr/bin/kubectl
chmod +x /usr/bin/kubectl
```

## 3. docker-ce
由于国内无法直接通过包管理器下载docker-ce，需要先配置国内yum源
```bash
# Step 1: 添加软件源信息
yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
# Step 2: 修改repo文件
sed -i 's+download.docker.com+mirrors.aliyun.com/docker-ce+' /etc/yum.repos.d/docker-ce.repo
# Step 3: 更新cache并安装Docker-CE
yum makecache fast
yum -y install docker-ce
```

# [Calico](https://github.com/projectcalico/calico)  
## 1. 部署Calico插件
启动Minikube  
```bash
minikube start --driver=none --cni=calico  
```
minikube会尝试从官方仓库中下载calico插件，如果因网络原因无法下载，需要先下载calico到本地，再启动minikube
```bash
curl -L https://github.com/projectcalico/calico/releases/download/v3.23.1/calicoctl-linux-amd64 -o /usr/local/bin/kubectl-calico
chmod +x /usr/local/bin/kubectl-calico
minikube start --driver=none --network-plugin=cni --cni=calico
```
观察calico的pod状态是否正常
```bash
watch kubectl get pods -l k8s-app=calico-node -A  
```
如果pod不断重启，状态不是Running，原因可能是由于默认网卡名称和本地网卡名称不符，需要将interface修改为本地网卡的名字，例如ens.*
```bash
kubectl edit daemonset calico-node -n kube-system  
```
## 2. 配置ippool
确认calico正常后，启动ippool。需要先编写ippool的配置文件
```yaml
apiVersion: projectcalico.org/v3
kind: IPPool
metadata:
  name: default-ipv4-ippool
spec:
  cidr: 192.170.0.0/25
  blockSize: 25
  ipipMode: Never
  natOutgoing: true
```
创建ippool
```bash
# 创建ippool
kubectl-calico apply -f pool.yaml  
# 查看ippool
kubectl-calico get ippool --allow-version-mismatch
```
如果在执行`kubectl-calico apply`命令过程中存在无法进行参数修改的报错，可能是默认ippool已经启动，可以删除后再进行创建  
```bash
# 查看ippool
kubectl-calico get ippool --allow-version-mismatch
NAME                  CIDR             SELECTOR   
default-ipv4-ippool   10.244.0.0/16   all()  
# default-ipv4-ippool已经缺省存在，进行删除
kubectl-calico --allow-version-mismatch delete ipp default-ipv4-ippool
# 使用编写的配置文件启动ippool
kubectl-calico --allow-version-mismatch apply -f pool.yaml 
# 检查
kubectl-calico get ippool --allow-version-mismatch
NAME                  CIDR             SELECTOR   
default-ipv4-ippool   192.170.0.0/25   all()      
```
查看calico分配的ip
```bash
kubectl calico get wep --all-namespaces --allow-version-mismatch
```
# 镜像
openGauss镜像及依赖都以openEuler为准进行演示，演示版本为3.0.0。可以根据需要调整相关依赖或版本
## 1. openEuler  
验证openEuler镜像是否能够满足正常启动，首先加载下载的镜像
```bash
wget http://121.36.97.194/openEuler-20.03-LTS-SP3/docker_img/x86_64/openEuler-docker.x86_64.tar.xz
docker image load -i openEuler-docker.x86_64.tar.xz
```
启动openEuler镜像
```bash
docker run -itd openeuler-20.03-lts-sp3 /bin/bash
```
进入container验证安装需要的软件，配置依赖
```bash
# 进入container
docker start -i 4500fd1c93bd
# 关闭TMOUT自动登出
echo 'unset TMOUT' >> /etc/bashrc
# 安装需要的软件
yum -y install sudo which bzip2 numactl-devel libaio libaio-devel readline-devel net-tools psmisc io
yum -y install lrzsz iputils libnsl
# 设置依赖readline
ln -s  /usr/lib64/libreadline.so.8 /usr/lib64/libreadline.so.6
```
## 2. openGauss
上述步骤都可以正常执行，确定openEuler镜像验证无误后，将上述步骤添加并编写dockerfile配置文件`og3.0.0-x86_64.dockerfile`。可以根据实际需求修改openGauss版本、架构、系统OS镜像信息。
```yaml
FROM openeuler-20.03-lts-sp3:latest

# docker build -f og2.0.1-x86_64.dockerfile -t opengauss-3.0.0:latest .

COPY tini-amd64 /usr/local/bin/tini
COPY scriptrunner_x86 /usr/local/bin/scriptrunner
COPY openGauss-3.0.0-openEuler-64bit.tar.bz2 openGauss-3.0.0-openEuler-64bit.tar.bz2
# COPY filebeat-7.11.1-linux-x86_64.tar.gz /gauss/files/filebeat-7.11.1-linux-x86_64.tar.gz
ADD filebeat-7.11.1-linux-x86_64.tar.gz /opt

ENV LANG en_US.UTF-8
ENV TZ Asia/Shanghai
ENV GAUSSHOME /gauss/openGauss/app
ENV PATH /gauss/openGauss/app/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ENV LD_LIBRARY_PATH /gauss/openGauss/app/lib
ENV PGDATA /gaussdata/openGauss/db1
ENV PGHOST /gauss/openGauss/tmp
ENV GAUSSLOG /gaussarch/log/omm
ENV GAUSS_ENV 2
ENV GS_CLUSTER_NAME openGauss

RUN yum -y install sudo which bzip2 numactl-devel libaio libaio-devel readline-devel net-tools psmisc iotop lrzsz iputils libnsl && \ 
    yum clean all && \
    echo 'unset TMOUT' >> /etc/bashrc && \
    ln -s  /usr/lib64/libreadline.so.8 /usr/lib64/libreadline.so.6 && \
    ln -s  /usr/lib64/libreadline.so.8 /usr/lib64/libreadline.so.7 && \
    echo 'omm ALL=(ALL) NOPASSWD: ALL' >> /etc/sudoers && \
    groupadd -g 580 dbgrp && \
    useradd -g dbgrp -u 580 omm && \
    mkdir -p /gauss/openGauss/app && \
    mkdir -p /gauss/openGauss/tmp && \
    mkdir -p /gaussdata/openGauss/db1 && \
    mkdir -p /gaussarch && \
    tar xf openGauss-3.0.0-openEuler-64bit.tar.bz2 -C /gauss/openGauss/app && \
    rm -rf openGauss-3.0.0-openEuler-64bit.tar.bz2 && \
    chown -R omm:dbgrp /gaussarch && \
    chown -R omm:dbgrp /gaussdata && \
    chown -R omm:dbgrp /gauss/openGauss && \
    mv /opt/filebeat-7.11.1-linux-x86_64 /opt/filebeat && \
    ln -s /opt/filebeat /opt/filebeat-7.11.1 && \
    chown -R omm:dbgrp /opt/filebeat && \
    chmod -R 755 /opt/filebeat && \
    chmod -R 755 /gaussarch && \
    chmod -R 755 /gauss/openGauss && \
    chmod +x /usr/local/bin/tini && \
    chmod +x /usr/local/bin/scriptrunner && \
    echo "export GAUSSHOME=/gauss/openGauss/app" >> /home/omm/.bashrc && \
    echo "export PATH=\$GAUSSHOME/bin:\$PATH " >> /home/omm/.bashrc && \
    echo "export LD_LIBRARY_PATH=\$GAUSSHOME/lib:" >> /home/omm/.bashrc && \
    echo "export PGDATA=/gaussdata/openGauss/db1" >> /home/omm/.bashrc && \
    echo "export PGHOST=/gauss/openGauss/tmp" >> /home/omm/.bashrc && \
    echo "export GAUSSLOG=/gaussarch/log/omm" >> /home/omm/.bashrc && \
    echo "export GAUSS_ENV=2" >> /home/omm/.bashrc && \
    echo "export GS_CLUSTER_NAME=openGauss" >> /home/omm/.bashrc

USER omm
```
使用编辑好的dockerfile打包openGauss镜像

打包镜像需要依赖`tini-amd64`, `scriptrunner_x86`, `filebeat-7.11.1-linux-x86_64.tar.gz` 以及 `openGauss-3.0.0-openEuler-64bit.tar.bz2`文件。 \
`tini-amd64`, `scriptrunner_x86`文件在当前仓库`openGauss-operator/execfiles`目录下。 \
`filebeat-7.11.1-linux-x86_64.tar.gz`下载地址参考下面第3步骤 *准备openGauss容器相关的介质和依赖*。 \
openGauss的镜像从社区官网下载极简版。

```bash
# 下载openGauss极简版，可以根据实际情况下载对应架构
curl -LO https://opengauss.obs.cn-south-1.myhuaweicloud.com/3.0.0/x86_openEuler/openGauss-3.0.0-openEuler-64bit.tar.bz2
# 打包镜像
docker build -f og3.0.0-x86_64.dockerfile -t opengauss-3.0.0:latest .
# 保存openGauss镜像
docker save opengauss-3.0.0:latest -o opengauss-docker.x86_64.tar.xz
```
镜像打包好后，运行并验证是否可用
```bash
# 运行镜像
docker run -itd opengauss-2.0.0:latest /bin/bash
docker exec -ti 4820abbe8980 /bin/bash
# 初始化openGauss实例，验证opengauss能用
gs_initdb --locale=en_US.UTF-8 -w omm@1234 --nodename=gaussdb -D  /gaussdata/openGauss/db1
```
## 3. operator
前面的验证步骤通过以后，可以开始部署operator。首先克隆openGauss operator到本地
```bash
git clone https://gitee.com/opengauss/openGauss-operator.git
```
进入到operator目录中
```bash
cd openGauss-operator
```
准备openGauss容器相关的介质和依赖
```bash
curl -LO https://opengauss.obs.cn-south-1.myhuaweicloud.com/3.0.0/x86_openEuler/openGauss-3.0.0-openEuler-64bit.tar.bz2
curl -L -O https://artifacts.elastic.co/downloads/beats/filebeat/filebeat-7.11.1-linux-x86_64.tar.gz
curl -LO https://github.com/krallin/tini/releases/download/v0.19.0/tini-amd64
```
制作operator镜像
```bash
make docker-build IMG=opengauss-operator:v0.1.0
```
operator的`./config/manager/kustomization.yaml`文件中，描述了operator镜像名以及版本号，需要根据实际情况进行修改。修改完成后进行deployment的部署，使用的namespace是opengauss-operator-system。
# 部署
## 1. 部署operator
部署openGauss operator
```bash
make deploy IMG=opengauss-operator:v0.1.0
```
查看结果
```bash
$ kubectl get deployment -n opengauss-operator-system
NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
opengauss-operator-controller-manager   1/1     1            1           13h
$ kubectl get po -n opengauss-operator-system
NAME                                                    READY   STATUS    RESTARTS   AGE
opengauss-operator-controller-manager-b9b8c6997-jjr2d   1/1     Running   0          8m39s
```
## 2. 部署openGauss集群
编辑openGauss集群的配置文件`sample.yaml`
```yaml
apiVersion: opengauss.sig/v1
kind: OpenGaussCluster
metadata:
  name: ogtest
spec:
  image: opengauss-3.0.0:lates
  readport: 30120
  writeport: 30121
  dbport: 26000 #根据实际情况进行修改
  localrole: primary
  #本地存储根路径，使用本地存储时填写
  hostpathroot: /root/opengauss_operator/hostpath 
  # pvc根据实际情况填写，太小可能会导致起不来
  storage: 10Gi 
  #storageclass: topolvm-provisioner
  #iplist 包括ip和node名称
  iplist:
  - ip: 192.170.0.11
    # nodename 修改为主机名
    nodename: control-plane.minikube.internal
  - ip: 192.170.0.12
    nodename: control-plane.minikube.internal
```
部署集群
```bash
kubectl apply -f sample.yaml 
```
检查openGauss部署情况
```bash
# 查看pod状态
$ kubectl get po -n default -o wide
NAME                          READY   STATUS    RESTARTS   AGE   IP             NODE                              NOMINATED NODE   READINESS GATES
og-ogtest-pod-192x170x0x11   2/2     Running   0          60m   192.170.0.11   control-plane.minikube.internal   <none>           <none>
og-ogtest-pod-192x170x0x12   2/2     Running   0          60m   192.170.0.12   control-plane.minikube.internal   <none>           <none>
# 查看IP分配
$  kubectl-calico --allow-version-mismatch get wep
WORKLOAD                      NODE                              NETWORKS          INTERFACE         
og-ogtest-pod-192x170x0x11   control-plane.minikube.internal   192.170.0.11/32   cali936d1825f4a   
og-ogtest-pod-192x170x0x12   control-plane.minikube.internal   192.170.0.12/32   calibfc706e677f   
# 查看PVC状态
$ kubectl get pvc
NAME                                   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
og-ogtest-pod-192x170x0x11-data-pvc   Bound    pvc-ba512962-92f3-4230-b2b1-9aa194d511a1   10Gi       RWO            standard       6d15h
og-ogtest-pod-192x170x0x11-log-pvc    Bound    pvc-ea74ee8c-166f-49e8-af63-132a4142ecbb   1Gi        RWO            standard       6d15h
og-ogtest-pod-192x170x0x12-data-pvc   Bound    pvc-cac90ea2-a397-4378-bfee-4d5758a911c1   10Gi       RWO            standard       6d15h
og-ogtest-pod-192x170x0x12-log-pvc    Bound    pvc-f4f789ff-d3d5-47e1-a35b-e5ac1e3b5827   1Gi        RWO            standard       6d15h
```
检查openGauss集群状态
```bash
# 进入openGauss的pod
$ kubectl exec -it og-ogtest-pod-192x170x0x11 -c og /bin/bash
# 使用`gs_ctl`命令查看集群状态
$ gs_ctl query -D /gaussdata/openGauss/db1/
[2022-06-08 09:39:36.868][7087][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
        sender_pid                     : 883
        local_role                     : Primary
        peer_role                      : Standby
        peer_state                     : Normal
        state                          : Streaming
        sender_sent_location           : 0/4000B18
        sender_write_location          : 0/4000B18
        sender_flush_location          : 0/4000B18
        sender_replay_location         : 0/4000B18
        receiver_received_location     : 0/4000B18
        receiver_write_location        : 0/4000B18
        receiver_flush_location        : 0/4000B18
        receiver_replay_location       : 0/4000B18
        sync_percent                   : 100%
        sync_state                     : Sync
        sync_priority                  : 1
        sync_most_available            : Off
        channel                        : 192.170.0.11:26002-->192.170.0.12:53786

 Receiver info:      
No information 
```
登陆数据库进行查询操作
```sql
$ gsql -r postgres -p 26000
gsql ((openGauss 3.0.0 build 980e5a05) compiled at 2022-04-07 10:15:57 commit 0 last mr  )
Non-SSL connection (SSL connection is recommended when requiring high-security)
Type "help" for help.

openGauss=# select version();
                                                                       version                                                                        
------------------------------------------------------------------------------------------------------------------------------------------------------
 (openGauss 3.0.0 build 980e5a05) compiled at 2022-04-07 10:15:57 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC) 7.3.0, 64-bit
(1 row)
```