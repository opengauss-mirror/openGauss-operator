# operator部署文档
本文档以minikube为基础，展示如何部署并使用operator。  
文档根据以下系统环境及依赖进行编写：
> CentOS 7.6  
> Minikube 1.20.0  
> Docker 20.10.16  
> openEuler 20.03 TLS  
> openGauss 5.0.2
>
> go  1.17.5

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
curl -L https://dl.k8s.io/release/v1.20.0/bin/linux/amd64/kubectl -o /usr/bin/kubectl
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

yum 安装 conntrack 

```
yum -y install conntrack
```

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
如果pod不断重启，状态不是Running，pod日志显示`Unable to auto-detect an IPv4 address using interface regexes [eth.*]: no valid host interfaces found`，原因可能是由于默认网卡名称和本地网卡名称不符，需要将interface修改为本地网卡的名字，例如ens.*
```bash
kubectl set env daemonset/calico-node -n kube-system IP_AUTODETECTION_METHOD=interface=ens*,eth.*
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
kubectl-calico get wep --all-namespaces --allow-version-mismatch
```
# 镜像

用于演示的openGauss版本为5.0.2，以openEuler-20.03作为镜像的基础发行版。可以根据需要调整相关依赖或版本。
## 1. openEuler  
首先从官方镜像站下载镜像，并加载到docker中。
```bash
wget https://repo.openeuler.org/openEuler-20.03-LTS-SP3/docker_img/x86_64/openEuler-docker.x86_64.tar.xz
docker load -i openEuler-docker.x86_64.tar.xz
```
加载后的镜像标签为：`openeuler-20.03-lts-sp3:latest`。

## 2. openGauss

打包镜像需要依赖`tini-amd64`, `scriptrunner_x86`, `filebeat-7.16.1-linux-x86_64.tar.gz` 以及 `openGauss-5.0.2-openEuler-64bit.tar.bz2`文件。

```
# 官网下载openGauss安装极简版介质（如下载企业版，还需对应修改Dockerfile文件）
wget https://opengauss.obs.cn-south-1.myhuaweicloud.com/5.0.2/x86_openEuler/openGauss-5.0.2-openEuler-64bit.tar.bz2
wget https://opengauss.obs.cn-south-1.myhuaweicloud.com/5.0.2/x86_openEuler/openGauss-5.0.2-openEuler-64bit-symbol.tar.gz

# 下载 scriptrunner_x86。该文件在当前仓库`openGauss-operator/execfiles`目录下
wget https://gitee.com/opengauss/openGauss-operator/blob/master/execfiles/scriptrunner_x86

# 下载 filebeat-7.16.1
curl -LO https://artifacts.elastic.co/downloads/beats/filebeat/filebeat-7.16.1-linux-x86_64.tar.gz

# 下载 tini-amd64
curl -LO https://github.com/krallin/tini/releases/download/v0.19.0/tini-amd64

# 构建openGauss镜像
docker build -f og5.0.2-x86_64.dockerfile -t opengauss-5.0.2:latest .
```

用于构建openGauss镜像的dockerfile配置文件`og5.0.2-x86_64.dockerfile`：（可以根据实际需求修改openGauss版本、架构、系统OS镜像信息。）

```
# 5.0.2 image build
FROM openeuler-20.03-lts-sp3:latest

# docker build -f og5.0.2-x86_64.dockerfile -t opengauss-5.0.2:latest .
COPY scriptrunner_x86 /usr/local/bin/scriptrunner
COPY openGauss-5.0.2-openEuler-64bit-all.tar.gz .
COPY openGauss-5.0.2-openEuler-64bit-symbol.tar.gz .
COPY tini-amd64 /usr/local/bin/tini
ADD filebeat-7.16.1-linux-x86_64.tar.gz /opt

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

RUN yum -y install sudo hostname which bzip2 numactl-devel libaio libaio-devel readline-devel net-tools psmisc coreutils iotop perf sysstat iperf3 lrzsz iputils libnsl && \ 
 echo 'omm ALL=(ALL) NOPASSWD: ALL' >> /etc/sudoers && \
 groupadd -g 580 dbgrp && \
 useradd -g dbgrp -u 580 omm && \
 mkdir -p /gauss/openGauss/app && \
 mkdir -p /gauss/openGauss/tmp && \
 mkdir -p /gaussdata/openGauss/db1 && \
 mkdir -p /gaussarch && \
 mkdir -p /gauss/files && \
 tar xf openGauss-5.0.2-openEuler-64bit-all.tar.gz -C /gauss/openGauss/app && \
 tar xf openGauss-5.0.2-openEuler-64bit-symbol.tar.gz && \
 cp -R symbols/* /gauss/openGauss/app && \
 rm -rf symbols && \
 rm -rf openGauss-5.0.2-openEuler-64bit-all.tar.gz && \
 rm -rf openGauss-5.0.2-openEuler-64bit-symbol.tar.gz && \
 chown -R omm:dbgrp /gaussarch && \
 chown -R omm:dbgrp /gaussdata && \
 chown -R omm:dbgrp /gauss/openGauss && \
 chown -R omm:dbgrp /gauss/files && \
 chmod -R 755 /gauss/openGauss && \
 chmod -R 755 /gaussarch && \
 chmod +x /usr/local/bin/scriptrunner &&\ 
 mv /opt/filebeat-7.16.1-linux-x86_64 /opt/filebeat && \
 ln -s /opt/filebeat /opt/filebeat-7.16.1 && \
 chown -R omm:dbgrp /opt/filebeat && \
 chmod 755 /opt/filebeat && \
 chmod +x /usr/local/bin/tini && \
 yum clean all && \
 echo "export GAUSSHOME=/gauss/openGauss/app" >> /home/omm/.bashrc && \ 
 echo "export PATH=\$GAUSSHOME/bin:\$PATH " >> /home/omm/.bashrc && \
 echo "export CMBC_SCRIPT_PATH=/cmbc_admin/openGauss" >> /home/omm/.bashrc && \
 echo "export PATH=\$CMBC_SCRIPT_PATH:\$PATH " >> /home/omm/.bashrc && \
 echo "export CMBC_ADMIN_PATH=/cmbc_admin" >> /home/omm/.bashrc && \
 echo "export PATH=\$CMBC_ADMIN_PATH:\$PATH " >> /home/omm/.bashrc && \
 echo "export LD_LIBRARY_PATH=\$GAUSSHOME/lib:\$LD_LIBRARY_PATH" >> /home/omm/.bashrc && \
 echo "export PGDATA=/gaussdata/openGauss/db1" >> /home/omm/.bashrc && \
 echo "export PGHOST=/gauss/openGauss/tmp" >> /home/omm/.bashrc && \
 echo "export GAUSSLOG=/gaussarch/log/omm" >> /home/omm/.bashrc && \
 echo "export GAUSS_ENV=2" >> /home/omm/.bashrc && \
 echo "export GS_CLUSTER_NAME=openGauss" >> /home/omm/.bashrc && \
 echo "alias ls='ls --color=auto'" >> /home/omm/.bashrc && \
 echo "alias gquery='gs_ctl query -D /gaussdata/openGauss/db1 '" >> /home/omm/.bashrc && \
 echo "alias loggings='gsql -d postgres -p 26000 -r'" >> /home/omm/.bashrc && \ 
 source /home/omm/.bashrc && \ 
 echo "alias ll='ls -l --color=auto'" >> /home/omm/.bashrc

USER omm
```

镜像打包好后，运行并验证是否可用
```bash
# 运行镜像
docker run -it opengauss-5.0.2:latest /bin/bash

# 初始化openGauss实例，验证opengauss能用
gs_initdb --locale=en_US.UTF-8 -w omm@1234 --nodename=gaussdb -D  /gaussdata/openGauss/db1
```
## 3. operator

前面的验证步骤通过以后，可以开始构建operator镜像

1. 准备go环境

```
wget https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
tar -C /usr/local -xf  go1.17.5.linux-amd64.tar.gz

echo "export GOROOT=/usr/local/go" >> /etc/profile
echo "export PATH=\$PATH:\$GOROOT/bin" >> /etc/profile
source /etc/profile
go version 
# 设置 GOPROXY
go env -w GOPROXY=https://goproxy.cn,direct
```

2. 克隆openGauss operator到本地并进入项目根目录：

```bash
git clone https://gitee.com/opengauss/openGauss-operator.git
```
3. 构建operator镜像

```bash
cd openGauss-operator
make docker-build IMG=opengauss-operator:v2.0.0
```
# 部署
## 1. 部署operator
1）部署openGauss operator

```bash
make deploy IMG=opengauss-operator:v2.0.0
```
> 注意：此处的镜像为上个步骤生成的镜像
>
> 使用make deploy命令时，需要修改源码路径(config/manager/kustomization.yaml)的image的newName和newTag，定义了部署资源时operator镜像名以及版本号，需要根据实际情况进行修改。修改完成后进行deployment的部署，使用的namespace是opengauss-operator-system。

若部署operator过程中报错：

```
[root@node openGauss-operator-master]# make deploy IMG=opengauss-operator:v2.0.0
/openGauss-operator-master/bin/controller-gen "crd:trivialVersions=true,crdVersions=v1" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
#cd config/manager && /docker/package/image/operator-img/openGauss-operator-master-latest/bin/kustomize edit set image controller=opengauss-operator:v2.0.0
/openGauss-operator-master/bin/kustomize build config/default | kubectl apply -f -
bash: kubectl: command not found
make: *** [deploy] Error 127
```

若在服务器上执行`kubectl`命令正常，则执行报错的语句：

```
[root@node openGauss-operator-master]# /openGauss-operator-master/bin/kustomize build config/default | kubectl apply -f - 
namespace/opengauss-operator-system unchanged
customresourcedefinition.apiextensions.k8s.io/opengaussclusters.opengauss.sig configured
role.rbac.authorization.k8s.io/opengauss-operator-leader-election-role configured
clusterrole.rbac.authorization.k8s.io/opengauss-operator-manager-role configured
rolebinding.rbac.authorization.k8s.io/opengauss-operator-leader-election-rolebinding unchanged
clusterrolebinding.rbac.authorization.k8s.io/opengauss-operator-manager-rolebinding unchanged
configmap/opengauss-operator-manager-config created
service/opengauss-operator-svc unchanged
statefulset.apps/opengauss-operator-controller-manager configured
```

2）查看结果

```bash
# kubectl get pod -n opengauss-operator-system 
NAME                                      READY   STATUS    RESTARTS   AGE
opengauss-operator-controller-manager-0   1/1     Running   0          7m48s
```
## 2. 部署openGauss单节点

1）部署集群

```bash
kubectl apply -f sample.yaml 
```
编写openGauss集群的配置文件（或直接使用项目目录下的示例`sample.yaml`），并启动集群：

```
apiVersion: opengauss.sig/v1
kind: OpenGaussCluster
metadata:
  name: ogtest01     #pod名，根据实际情况修改
spec:
  cpu: '2'
  storage: 10Gi
  image: opengauss-5.0.2:latest
  memory: 3G
  readport: 31000
  writeport: 31001
  localrole: primary
  dbport: 26000
  hostpathroot: /docker/opengauss_operator/ogtest01    #本地存储根路径，使用本地存储时填写
  #storageclass: topolvm-class    #填写对应存储插件的storageclass即可
  networkclass: calico     #当前仅支持calico和kube-ovn两种
  config:
    advance_xlog_file_num: "10"
    archive_command: '''cp %p /gaussarch/archive/%f'''
    archive_dest: '''/gaussdata/archive/archive_xlog'''
    archive_mode: "on"
    bbox_dump_path: '''/gaussarch/corefile'''
    log_directory: '''/gaussarch/log/omm/pg_log'''
    max_connections: "2000"
  iplist:
  - nodename: node    # 宿主机hostname名
    ip: 192.170.0.110  # POD IP，根据实际情况修改
```

2）检查openGauss部署情况

```bash
# 查看pod状态
$ kubectl get pod -n default -o wide
NAME                            READY   STATUS    RESTARTS   AGE    IP              NODE   NOMINATED NODE   READINESS GATES
og-ogtest01-pod-192x170x0x110   2/2     Running   0          105m   192.170.0.110   node   <none>           <none>  <none>           <none>

# 查看IP分配
$ kubectl-calico --allow-version-mismatch get wep
WORKLOAD                        NODE   NETWORKS           INTERFACE         
og-ogtest01-pod-192x170x0x110   node   192.170.0.110/32   cali31ff0028558   

# 查看PVC状态
$ kubectl get pvc
NAME                                     STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
og-ogtest01-pod-192x170x0x110-data-pvc   Bound    pvc-28efb88b-4bea-435d-8b64-5bde1609995e   10Gi       RWO            standard       105m
og-ogtest01-pod-192x170x0x110-log-pvc    Bound    pvc-63802105-b146-47e2-9805-0cb424a4c131   1Gi        RWO            standard       105m
```
3）检查openGauss集群状态

```bash
# 进入openGauss的pod
$ kubectl exec -it og-ogtest01-pod-192x170x0x110 -c og /bin/bash
# 使用`gs_ctl`命令查看集群状态
$ gs_ctl query -D /gaussdata/openGauss/db1/
[2024-06-17 11:36:18.851][31041][][gs_ctl]: gs_ctl query ,datadir is /gaussdata/openGauss/db1 
 HA state:           
        local_role                     : Primary
        static_connections             : 1
        db_state                       : Normal
        detail_information             : Normal

 Senders info:       
No information 
 Receiver info:      
No information 
```
4）登陆数据库进行查询操作

```sql
$ gsql -p 26000 postgres -r 
gsql ((openGauss 5.0.2 build 48a25b11) compiled at 2024-05-14 10:53:45 commit 0 last mr  )
Non-SSL connection (SSL connection is recommended when requiring high-security)
Type "help" for help.

openGauss=# select version() ;
                                                                       version                                                        
                
--------------------------------------------------------------------------------------------------------------------------------------
----------------
 (openGauss 5.0.2 build 48a25b11) compiled at 2024-05-14 10:53:45 commit 0 last mr   on x86_64-unknown-linux-gnu, compiled by g++ (GCC
) 7.3.0, 64-bit
(1 row)
```

