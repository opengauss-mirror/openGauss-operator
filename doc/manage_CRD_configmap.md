# 配置configmap
openGauss operator部署集群时，支持2个可配置的configmap，对应的cr属性分别为scriptconfig和filebeatconfig
## scriptconfig
scriptconfig对应自定义任务执行脚本的configmap,默认配置名称**opengauss-script-config**,支持自定义配置脚本
  >scriptconfig-og.ini/scriptconfig-sidecar.ini中配置脚本将在哪个容器中执行
  scriptconfig-og.ini配置表示在og container中执行；scriptconfig-sidecar.ini配置表示在sidecar container中执行 
  任务设置格式如下
  (crontab表达式) 脚本路径 [脚本参数]

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${SCRIPT_CM_NAME}
  namespace: ${CR_NAMESPACE}
  labels:
    app.kubernetes.io/app: opengauss
data:
  scriptconfig-og.ini: |
    #任务设置
    #(crontab表达式) 脚本路径 [脚本参数]
    
    #每小时一次，清理超过24小时的idle事务
    (0 * * * *) /gauss/files/script/clean-idle-transaction.sh 24
  
  scriptconfig-sidecar.ini: |
    #任务设置
    #(crontab表达式) 脚本路径 [脚本参数]
    
    #每天凌晨2点，在目录/gaussarch/log，清理3天以上的日志
    (0 2 * * *) /gauss/files/script/clean-log.sh /gaussarch/log 3
    
  clean-idle-transaction.sh: |
    #!/usr/bin/env bash
    PGDATA="${PGDATA%*/}"
    status=$(gs_ctl status -D ${PGDATA})
    role="$(echo "${status}" | grep -E -i 'primary|standby' | awk -F ' ' '{print $5}' | awk -F '"' '{print $2}')"
    if [ "${role}" == "primary" ]; then
        logpath="/gauss/files/logs/clean-idle-transaction.log"
        idlelimit=$1
        timestamp=$(date "+%Y-%m-%d %H:%M:%S")
        echo -e "$timestamp clean transactions idled for $idlelimit hour, sql is: gsql -d postgres -p 26000 -c \"select pg_terminate_backend(sessionid) from pg_stat_activity where state='idle in transaction' and current_timestamp - state_change > interval '$1 hour';\" \n" >> ${logpath}
        gsql -d postgres -p 26000 -c "select pg_terminate_backend(sessionid) from pg_stat_activity where state='idle in transaction' and current_timestamp - state_change > interval '$1 hour';"  >> ${logpath}
    fi
    
  clean-log.sh: |
    #!/usr/bin/env bash
    typeset log_dir=$1
    typeset retain=$2
    typeset today=$(date "+%Y%m%d%H%M")
    logs=${log_dir}/delete_opgslog_${today}.log
    lsts=${log_dir}/delete_opgslog_${today}.lst
    find /gaussarch/log -type f -mtime +1 -name *log -exec gzip {} \; 1> ${logs} 2>${logs}
    find /gaussarch/log -type f -mtime +${retain} 1> ${lsts} 2>${lsts}
    find /gaussarch/log -type f -mtime +${retain} -exec rm -f {} \; 1>> ${logs} 2>> ${logs}
    timestamp=$(date "+%Y-%m-%d %H:%M:%S")
    echo "$timestamp clean logs in $log_dir older than $retain days complete" >> /gauss/files/logs/clean-log.log
```
# filebeatconfig 
filebeatconfig对应执行脚本的configmap,默认配置名称：**opengauss-filebeat-config**,支持自定义配置，将日志通过filebeat转发到es或其他.
默认配置转到到es，设置如下，部署时需要根据实际情况修改

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <filebe>
  namespace: ${CR_NAMESPACE}
  labels:
    app.kubernetes.io/app: opengauss
data:
  filebeat.opengauss.yml: |
    filebeat.inputs:
    - type: log
      enabled: true
      paths:
      - /gaussarch/*audit.log
      tags: "auditlog"
      fields:
        podIp: ${MY_POD_IP}
        clusterName: ${CR_NAME}
      fields_under_root: true
    - type: log
      enabled: true
      paths:
      - /gaussarch/log/omm/pg_log/*.log
      tags: "syslog"
      fields:
        podIp: ${MY_POD_IP}
        clusterName: ${CR_NAME}
      fields_under_root: true
    - type: log
      enabled: true
      paths:
      - /gaussarch/log/omm/bin/*/*.log
      tags: "omlog"
      fields:
        podIp: ${MY_POD_IP}
        clusterName: ${CR_NAME}
      fields_under_root: true
      multiline.type: pattern
      multiline.negate: true
      multiline.pattern: '^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\]'
      multiline.match: after
    output.elasticsearch:
      hosts: ["xxx.xxx.xxx.xxx:port"]
      username: "elastic"
      password: "elastic"
      protocol: "http"
      pipelines:
      - pipeline: "filebeat-opengauss-syslog"
        when.contains:
          tags: "syslog"
      - pipeline: "filebeat-opengauss-omlog"
        when.contains:
          tags: "omlog"
      - pipeline: "filebeat-opengauss-auditlog"
        when.contains:
          tags: "auditlog"
      indices:
      - index: "filebeat-opengauss-syslog-%{+YYYY.MM}"
        when.contains:
          tags: "syslog"
      - index: "filebeat-opengauss-omlog-%{+YYYY.MM}"
        when.contains:
          tags: "omlog"
      - index: "filebeat-opengauss-auditlog-%{+YYYY.MM}"
        when.contains:
          tags: "auditlog"
```
以上两个自定义的configmap配置，具体可以在yaml.go中找到.