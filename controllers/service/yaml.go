/*
Copyright (c) 2021 opensource@cmbc.com.cn
OpenGauss Operator is licensed under Mulan PSL v2.
You can use this software according to the terms and conditions of the Mulan PSL v2.
You may obtain a copy of Mulan PSL v2 at:
         http://license.coscl.org.cn/MulanPSL2
THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
See the Mulan PSL v2 for more details.
*/

package service

import (
	"strings"

	opengaussv1 "opengauss-operator/api/v1"
)

const (
	CR_NAME             = "CR_NAME"
	CR_NAMESPACE        = "CR_NAMESPACE"
	CR_API_VERSION      = "CR_API_VERSION"
	CR_KIND             = "CR_KIND"
	CR_UID              = "CR_UID"
	CR_SECRET_NAME      = "CR_SECRET_NAME"
	CR_BACKUP_PATH      = "CR_BACKUP_PATH"
	CR_SVC_PORT         = "CR_SVC_PORT"
	CR_DB_PORT          = "CR_DB_PORT"
	SVC_NAME            = "SVC_NAME"
	DB_ROLE             = "DB_ROLE"
	POD_NAME            = "POD_NAME"
	PV_TYPE             = "PV_TYPE"
	PV_STORAGE_CAPACITY = "PV_STORAGE_CAPACITY"
	PV_NODE_SELECT      = "PV_NODE_SELECT"
	PV_STORAGE_CLASS    = "PV_STORAGE_CLASS"
	PVC_STORAGE_LMT     = "PVC_STORAGE_LMT"
	PVC_STORAGE_REQ     = "PVC_STORAGE_REQ"
	PVC_STORAGE_CLASS   = "PVC_STORAGE_CLASS"
	PVC_TYPE            = "PVC_TYPE"
	VOLUME_TYPE_DATA    = "data"
	VOLUME_TYPE_LOG     = "log"

	POD_IP                               = "POD_IP"
	POD_DB_IMG                           = "POD_DB_IMG"
	DB_CPU_LMT                           = "DB_CPU_LMT"
	DB_CPU_REQ                           = "DB_CPU_REQ"
	DB_MEM_LMT                           = "DB_MEM_LMT"
	DB_MEM_REQ                           = "DB_MEM_REQ"
	POD_SIDECAR_IMG                      = "POD_SIDECAR_IMG"
	SIDECAR_CPU_LMT                      = "SIDECAR_CPU_LMT"
	SIDECAR_CPU_REQ                      = "SIDECAR_CPU_REQ"
	SIDECAR_MEM_LMT                      = "SIDECAR_MEM_LMT"
	SIDECAR_MEM_REQ                      = "SIDECAR_MEM_REQ"
	POD_NODE_SELECT                      = "POD_NODE_SELECT"
	HOSTPATH_ROOT                        = "HOSTPATH_ROOT"
	CR_ARCHIVE_PATH                      = "CR_ARCHIVE_PATH"
	READ_SVC_NAME                        = "READ_SVC_NAME"
	WRITE_SVC_NAME                       = "WRITE_SVC_NAME"
	CLUSTER_CM_NAME                      = "CLUSTER_CM_NAME"
	SCRIPT_CM_NAME                       = "SCRIPT_CM_NAME"
	FILEBEAT_CM_NAME                     = "FILEBEAT_CM_NAME"
	CLUSTER_CM_VAL                       = "CLUSTER_CM_VAL"
	SCRIPT_CM_VAL                        = "SCRIPT_CM_VAL"
	FILEBEAT_CM_VAL                      = "FILEBEAT_CM_VAL"
	GRACE_PERIOD                         = "GRACE_PERIOD"
	TOLERATION_SECOND                    = "TOLERATION_SECOND"
	NEED_INIT                            = "NEED_INIT"
	LIVENESS_PROBE_PERIOD                = "LIVENESS_PROBE_PERIOD"
	READINESS_PROBE_PERIOD               = "READINESS_PROBE_PERIOD"
	FILEBEAT_OUT_KAFKA_HOSTS_KEY         = "FILEBEAT_CM_OUT_KAFKA_HOSTS"
	FILEBEAT_OUT_KAFKA_USERNAME_KEY      = "FILEBEAT_CM_OUT_KAFKA_USERNAME"
	FILEBEAT_OUT_KAFKA_PASSWORD_KEY      = "FILEBEAT_CM_OUT_KAFKA_PASSWORD"
	FILEBEAT_OUT_KAFKA_TOPIC_KEY         = "FILEBEAT_CM_OUT_KAFKA_TOPIC"
	DEFAULT_FILEBEAT_OUT_KAFKA_HOSTS_VAL = "kafkagw.xxx.xxx.xxx:9092"
	YAML_FILEBEAT_CM                     = `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${FILEBEAT_CM_NAME}
  namespace: ${CR_NAMESPACE}
  labels:
    app.kubernetes.io/app: opengauss
data:
  filebeat.opengauss.yml: |
    filebeat.inputs:
    - type: log
      id: auditlog-input
      enabled: true
      paths:
      - /gaussarch/*audit.log
      tags: "auditlog"
      fields:
        podIp: ${MY_POD_IP}
        clusterName: ${CR_NAME}
      fields_under_root: true
    - type: log
      id: syslog-input
      enabled: true
      paths:
      - /gaussarch/log/omm/pg_log/*.log
      tags: "syslog"
      fields:
        podIp: ${MY_POD_IP}
        clusterName: ${CR_NAME}
      fields_under_root: true
    - type: log
      id: omlog-input
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
      hosts: ["192.168.1.162:9200"]
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
  `
	YAML_SCRIPT_CM = `apiVersion: v1
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
    logs=${log_dir}/delete_opgslog.log
    lsts=${log_dir}/delete_opgslog.lst
    find /gaussarch/log -type f -mtime +1 -name \*log -exec gzip {} \; 1>> ${logs} 2>>${logs}
    find /gaussarch/log -type f -mtime +${retain} 1>> ${lsts} 2>>${lsts} 
    find /gaussarch/log -type f -mtime +${retain} -exec rm -f {} \; 1>> ${logs} 2>>${logs}
    timestamp=$(date "+%Y-%m-%d %H:%M:%S")
    echo "$timestamp clean logs in $log_dir older than $retain days complete" >> /gauss/files/logs/clean-log.log
  `
	YAML_CM = `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${CLUSTER_CM_NAME}
  namespace: ${CR_NAMESPACE}
  labels:
    app.kubernetes.io/app: opengauss
    app.kubernetes.io/name: ${CR_NAME}
  ownerReferences:
  - apiVersion: ${CR_API_VERSION}
    blockOwnerDeletion: true
    controller: true
    kind: ${CR_KIND}
    name: ${CR_NAME}
    uid: ${CR_UID}
data:
  sidecar-remove-log.sh: |
    #!/usr/bin/env bash
    find /gaussarch/log/omm/pg_log -type f -name '*.log' -mtime +3 -exec rm {} \;
    find /gaussarch/log/omm/bin -type f -name '*.log' -mtime +3 -exec rm {} \;
  sidecar-get-audit-log.sh: |
    #!/usr/bin/env bash

    # show audit_enabled
    # gs_guc set -D /gaussdata/openGauss/db1 -c "audit_enabled=on"
    touch /gaussarch/filebeat.audit.log
    while :
    do
        getTime=$(date '+%Y-%m-%d %H:%M:%S')
        getTime10sec=$(date --date="1 minutes ago" '+%Y-%m-%d %H:%M:%S')
        # getTime10sec=$(date --date="30 seconds ago" '+%Y-%m-%d %H:%M:%S')

        echo getLog= gsql -d postgres -c "select * from pg_query_audit('${getTime10sec}','${getTime}')"
        getLog=$(gsql -d postgres -c "select * from pg_query_audit('${getTime10sec}','${getTime}')" 2> /dev/null | tail -n +3 | head -n -2)

        # getLog=$(gsql -d postgres -c "select * from pg_query_audit('2021-10-01 08:00:00','2021-10-11 17:00:00')" 2> /dev/null | tail -n +3 | head -n -2)
        # echo "${getLog}"
        echo "${getLog}" >> /gaussarch/filebeat.audit.log
        sleep 60
    done

  og-restore.sh: |
    #!/usr/bin/env bash
    # 必须使用omm用户进行, 仅适用于一个主机一个og进程
    syntax() {
      echo "bash $0  -backupFile /gaussdata/backup/20211227221002.tar  -dataDir /gaussdata/data/db1"
      exit 22
    }
    # 初始化输入的参数
    for inopt in $@
    do
      case $(echo $inopt | tr a-z A-Z) in
        -BACKUPFILE) CurOpt="-BACKUPFILE";continue;;
        -DATADIR) CurOpt="-DATADIR";continue;;
        -LOGLEVEL|-L) CurOpt="-LOGLEVEL";continue;;
        -HELP|-H) CurOpt="HELP";syntax;;
        -*) CurOpt="";continue;;
      esac

      case "${CurOpt}" in
        -BACKUPFILE) typeset backupFile="${inopt}";continue;;
        -DATADIR) typeset dataDir="${inopt}";continue;;
        -LOGLEVEL) typeset -i LogLevel=${inopt:-3};continue;;
      esac
    done
    # 默认参数
    dataDir="${dataDir:-/gaussdata/openGauss/db1}"
    start_time=$(date "+%Y%m%d%H%M%S")
    
    # 日志输出函数
    f_PrintLog() {
      case $(echo "$1"|tr "a-z" "A-Z") in
        DEBUG) typeset _LogLevel=4;typeset _LogFlag="DEBUG";;
        INFO|INFO_) typeset _LogLevel=3;typeset _LogFlag="INFO_";;
        WARN|WARN_) typeset _LogLevel=2;typeset _LogFlag="WARN_";;
        ERROR|STOP|STOP_) typeset _LogLevel=1;typeset _LogFlag="ERROR";;
        SUCC|SUCC_|SUCCESS) typeset _LogLevel=0;typeset _LogFlag="SUCC_";;
      esac
      if [ ${_LogLevel:-3} -le ${LogLevel:-3} ]
      then
        typeset LogFile="${LogFile:-"/tmp/${PShellName:-${ShellName}}.log"}"
        touch "${LogFile}" 2>/dev/null
        if [ -w "${LogFile}" ]
        then
            echo "[${_LogFlag}]:$(date "+%Y%m%d.%H%M%S"):${UserName}@${HostName}:${2}"|tee -a "${LogFile}";
        else
            echo "[${_LogFlag}]:$(date "+%Y%m%d.%H%M%S"):${UserName}@${HostName}:${2}";
        fi
      fi
      # Here defined if scirpt encountered an error,then exit the script
      [ "${_LogFlag}" = "ERROR" ] && exit 55 || return 0;
    }
    chkCmdOK() {
      if [ "$?" -eq 0 ]; then
        f_PrintLog "$1" "$2"
      fi
    }
    chkCmdNO() {
      if [ "$?" -ne 0 ]; then
        f_PrintLog "$1" "$2"
      fi
    }
    # 处理绝对路径的表空间
    funcEditTbs() {
      #判断是否存在自定义表空间
      count_backup_tar=$(ls -rlt ${backup_base_path}/*tar* |wc -l)
      #如果tar包数量大于1,代表除base.tar还有自定义表空间tar包, 相反则代表没有自定义表空间,则退出函数
      if [ "${count_backup_tar}" -gt "1" ]; then
        # 解压表空间备份文件
        gzip -d ./*.tar.gz
        tablespaceArr=($(cat ${dataDir}/tablespace_map|tr ' ' ':'))
        f_PrintLog "INFO" "List of tablespaces:  ${tablespaceArr[@]} "
        for ((i = 0; i < ${#tablespaceArr[@]}; i++)); do
          getTbsOid=$(echo ${tablespaceArr[i]} | awk -F: '{print $1}' )
          getTbsPath=$(echo ${tablespaceArr[i]} | awk -F: '{print $2}' )
          str=pg_location
          res=$(echo ${getTbsPath}|grep "${str}")
          if [ "${res}" != ""  ]; then
            mkdir -p ${getTbsPath}
            gs_tar -F ${getTbsOid}.tar -D $getTbsPath

            else
            mv ${getTbsPath} ${getTbsPath}.bak.${start_time}
            mkdir -p ${getTbsPath}
            chmod 700 ${getTbsPath}
            gs_tar -F ${getTbsOid}.tar -D ${getTbsPath}
          fi
        done
      else
        f_PrintLog "INFO" "No custom tablespace exists and no action is required"
      fi
    }
    # 备份原数据目录
    if [ ! -f ${backupFile} ];then
      f_PrintLog "INFO" "Cannot find data file ${backupFile}, restore DB data failed, exit."
      exit 1
    fi
    mv $dataDir $dataDir$start_time
    # 创建数据目录,并附权(700),权限错误将不能启动数据库
    mkdir -p ${dataDir}
    chmod 700 ${dataDir}
    
    # 解压备份文件
    backup_path=${backupFile%/*}
    f_PrintLog "INFO" "The tar package is decompressed: tar -xf $backupFile -C ${backup_path}... "
    tar -xf ${backupFile} -C ${backup_path}
    chkCmdOK "SUCC" "Decompress is successful"
    
    # 解压base.tar文件至原数据目录
    backup_base_path=${backupFile%.*}
    cd ${backup_base_path}
    f_PrintLog "INFO" "The tar.gz package is decompressed: gzip -d base.tar.gz..."
    gzip -d base.tar.gz
    chkCmdOK "SUCC" "Decompress base.tar.gz is successful"
    f_PrintLog "INFO" "The base.tar package is decompressed: gs_tar -F base.tar -D ${dataDir}..."
    gs_tar -F base.tar -D ${dataDir}
    chkCmdOK "SUCC" "Decompress base.tar.gz is successful"
    
    # 处理自定义表空间
    funcEditTbs
    # 清理解压目录
    rm -rf ${backup_base_path}
    f_PrintLog "INFO" "Restore DB data complete."  
    
  sidecar-basebackup.sh: |
    #!/usr/bin/env bash
    backupRoot="/gaussdata/backup/"
    start_time=$(date "+%Y%m%d%H%M%S")
    backupPath=$backupRoot$start_time
    mkdir -p $backupPath
    chmod 700 $backupPath
    backuplog="/gauss/files/backup$start_time.log"
    
    gs_backup_status="prepare"
    gs_basebackup -D $backupPath -h 127.0.0.1 -p ${CR_DB_PORT} -F tar -z -X fetch >$backuplog 2>&1 &
    pid_basebackup_flag=$(ps aux |grep -i "gs_basebackup -D $backupPath -h 127.0.0.1" |grep -v grep |wc -l)
    
    #判断进程是否在, 每5s探测一次,一天内(即86400s)备份如果还没结束,则判断为超时
    if [ "$pid_basebackup_flag" -eq "1" ]; then
      i=0
      while [ "$i" -lt "86400" ]
      do
        pid_basebackup_flag=$(ps aux |grep -i "gs_basebackup -D $backupPath -h 127.0.0.1" |grep -v grep |wc -l)
        if [ "$pid_basebackup_flag" -eq "1" ];then
          gs_backup_status="running"
          echo "gs_basebackup is running for "$i"s"
        else
          gs_backup_status="finished"
          echo "gs_basebackup complete, total time cost is "$i"s"
          break
        fi
        sleep 5
        i=$((${i} + 5))
      done
      if [ "$gs_backup_status" == "finished" ]; then
        #如果进程结束, 则通过备份日志判断是否成功
        backup_finished_flag=$(tail -n 1 $backuplog |grep -i "base backup successfully" |wc -l)
      else
        gs_backup_status="timeout"
        pid_basebackup=$(ps aux |grep -i "gs_basebackup -D $backupPath -h 127.0.0.1" |grep -v grep |awk '{printf $2}')
        echo "Backup timeout after "$i"s, backup process is "$pid_basebackup", the process will be killed"
        kill -9 $pid_basebackup
      fi
    else
      gs_backup_status="failed"
      echo "gs_basebackup execute failed, Please check output in "$backuplog
    fi
    
    if [ "$backup_finished_flag" -eq "1" ]; then
      gs_backup_status="success"
      echo "gs_basebackup is successful"
    else
      gs_backup_status="failed"
      echo "gs_basebackup execute failed, Please check output in "$backuplog
    fi
    
    cd  $backupRoot
    tar -cvf $start_time.tar $start_time
    rm -rf $start_time
    backup_size=$(du -sb $backupRoot$start_time.tar |awk '{print $1}')
    end_time=$(date "+%Y%m%d%H%M%S")
    echo -e "start_time:$start_time \nend_time:$end_time \nbackup_status:$gs_backup_status \nbackup_file:$backupRoot$start_time.tar \nbackup_size=$backup_size">>"$backupRoot"gs_basebackup.log
  
  K8SChkRepl.sh: |
    #!/usr/bin/env bash
    syntax() {
        echo "bash $0 -user chkdb -password K8S@chkdb -dbName shindb -tableName chkrepl -basebackupLog /gauss/files/basebackup.log"
        exit 22
    }
    # 初始化输入的参数
    for inopt in $@; do
        case $(echo $inopt|tr a-z A-Z) in
            -USER) CurOpt="-USER";continue;;
            -PASSWORD) CurOpt="-PASSWORD";continue;;
            -DBNAME) CurOpt="-DBNAME";continue;;
            -TABLENAME) CurOpt="-TABLENAME";continue;;
            -BASEBACKUPLOG) CurOpt="-BASEBACKUPLOG";continue;;

            -LOGLEVEL|-L) CurOpt="-LOGLEVEL";continue;;
            -HELP|-H) CurOpt="HELP";syntax;;
            -*) CurOpt="";continue;;
        esac
        case "${CurOpt}" in
            -USER) typeset user="${inopt}";continue;;
            -PASSWORD) typeset password="${inopt}";continue;;
            -DBNAME) typeset dbName="${inopt}";continue;;
            -TABLENAME) typeset tableName="${inopt}";continue;;
            -BASEBACKUPLOG) typeset basebackupLog="${inopt}";continue;;

            -LOGLEVEL) typeset -i LogLevel=${inopt:-3};continue;;
        esac
    done

    # 参数默认值
    port="${port:-${CR_DB_PORT}}"
    user="${user:-chkdb}"
    password="${password:-K8S@chkdb}"
    dbName="${dbName:-shindb}"
    tableName="${tableName:-chkrepl}"
    basebackupLog="${basebackupLog:-/gauss/files/basebackup.log}"
    restoreLog="${restoreLog:-/gauss/files/restore.log}"
    # K8S ENV 环境变量 OG_PASSWORD
    # GS_PASSWORD="${GS_PASSWORD:-${OG_PASSWORD}}"
    PGDATA="${PGDATA%*/}"

    startTime=$(date '+%Y-%m-%d %H:%M:%S')
    ts=$(echo ${startTime} | tr -d '\- :')

    # MY_POD_IP 为k8s注入的环境变量
    host=${MY_POD_IP}

    # 初始变量
    export LC_ALL=en_US.UTF-8
    export LANG=en_US.UTF-8
    export TZ='Asia/Shanghai'
    ShellName="$(echo $0 | awk -F / '{print $NF}')"

    # 防重入
    MaxRunningCount=1
    psRes=$(ps -ef)
    ShellCount=$(echo "${psRes}" | grep "${ShellName}" | grep -vE "$$|^more|^vi|^view|^grep|^tail" | awk 'END{print NR}')
    if [ ${ShellCount} -gt ${MaxRunningCount} ]; then
        echo "$(date)" >> /gauss/files/ps.log
        ps -elf >> /gauss/files/ps.log
        echo "$$" >> /gauss/files/ps.log
        echo "----" >> /gauss/files/ps.log

        # 防重入需要告知前端，返回 {"running":1}
        echo '{"running":1}'
        exit
    fi

    ########################### 初始状态 ###########################
    # 0: 否
    # 1: 是
    # key value 统一小写
    # opengauss进程是否存在
    chkprocess=0
    # 是否正常连接
    chkconn=0
    # 是否处于维护模式
    maintenance=0
    # 是否 pending 状态
    pending=0
    # 是否为主节点
    primary=0
    # 是否为备节点
    standby=0
    # 是否为单节点
    standalone=0
    # 是否 hang 住
    hang=0
    # 主从状态是否正常
    chkrepl=0
    # build状态：0 build完成；1 失败；2 正在build；3 尚未开始build
    buildstatus=0
    # 备份状态：0 没有备份日志文件；1 成功；2 失败；3 备份中
    basebackup=0
    # 恢复状态：0 没有恢复日志文件；1 成功；2 失败；3 进行中
    restore=0
    # 配置连接
    connections=0
    # 详细信息
    detailinfo=''

    # 指定用户测试读写
    # create database shindb;
    # create table shindb.chkrepl (id int key, chktime datetime);
    # create user chkdb@'127.0.0.1' identified by 'K8S@chkdb';
    # -- DML权限
    # grant select, insert, delete on shindb.chkrepl to chkdb@'127.0.0.1';

    ########################### 开始检测 ###########################
    # 需要检测 build 状态，如果 build 失败，需要告警通知人工介入
    # 维护模式
    [ -f '/gauss/files/maintenance' ] && maintenance=1

    ps aux | grep -w 'bin/gaussdb' | grep -v grep > /dev/null && chkprocess=1

    # build状态：0 成功；1 失败；2 正在build
    querybuild=$(gs_ctl querybuild -D ${PGDATA} | grep -w -i 'db_state')
    echo "${querybuild}" | grep -w -i 'Building' > /dev/null && buildstatus=2
    echo "${querybuild}" | grep -w -i 'Build completed' > /dev/null && buildstatus=0
    [ "${buildstatus}" != 0 -a "${buildstatus}" != 2 ] && buildstatus=1

    # 只有进程存在时，才会判断状态
    if [ "${chkprocess}" = "1" ]; then
        res1=$(gs_ctl query -D ${PGDATA})
        if [ "$?" -eq 0 ]; then
            chkconn=1
            echo "${res1}" | grep -w -i 'db_state' | grep -w -i 'Normal' > /dev/null && chkrepl=1
            echo "${res1}" | grep -w -i 'local_role' | grep -w -i 'Normal' > /dev/null && standalone=1
            echo "${res1}" | grep -w -i 'local_role' | grep -w -i 'Pending' > /dev/null && pending=1
            echo "${res1}" | grep -w -i 'local_role' | grep -w -i 'Standby' > /dev/null && standby=1
            echo "${res1}" | grep -w -i 'local_role' | grep -w -i 'Primary' > /dev/null && primary=1
            connections=$(echo "${res1}" | grep -w -i 'static_connections' | grep -Eo '([0-9]+)')
            detailinfo=$(echo "${res1}" | grep -w -i 'detail_information' | grep -Eo ': ([a-zA-Z\s\.,]+)' | awk -F ': ' '{print $2}')
        fi
    fi

    # 备份状态：0 没有备份日志文件；1 成功；2 失败；3 备份中
    if [ -f "${basebackupLog}" ]; then
        tail -n1 ${basebackupLog} | grep -w -i 'successfully' > /dev/null && basebackup=1
        tail -n1 ${basebackupLog} | grep -E -i 'keepalive.*received' > /dev/null && basebackup=3
        [ "${basebackup}" != 1 -a "${basebackup}" != 3 ] && basebackup=2
    fi
    if [ -f "${restoreLog}" ]; then
        tail -n1 ${restoreLog} | grep -w -i 'complete' > /dev/null && restore=1
        tail -n1 ${restoreLog} | grep -E -i 'failed' > /dev/null && restore=2
        [ "${restore}" != 1 -a "${restore}" != 2 ] && restore=3
    fi

    echo "{\"chkprocess\":${chkprocess}, \"chkconn\":${chkconn}, \"maintenance\":${maintenance}, \"pending\":${pending}, \"primary\":${primary}, \"standby\":${standby}, \"standalone\":${standalone}, \"hang\":${hang}, \"chkrepl\":${chkrepl}, \"buildstatus\":${buildstatus}, \"basebackup\":${basebackup}, \"restore\":${restore}, \"connections\":${connections}, \"detailinfo\":\"${detailinfo}\"}"

    # 通过对指定表插入数据判断，暂时跳过高斯的hang检测
    # if [ "${chkconn}" = "1" -a "${pending}" = "0" ]; then
    #     timeout 1 gsql -U omm -W ${OG_PASSWORD} -d postgres -c "select name,setting from pg_settings where name ='default_transaction_read_only'" | grep -i default_transaction_read_only | grep -i -w off > /dev/null
    #     if [ "$?" -eq 0 ]; then
    #         # timeout 1 mysql -u${user} -p${password} -h127.0.0.1 -P${port} -e "replace into ${dbName}.${tableName} values (1, now())" 1> /dev/null 2>&1
    #         # [ "$?" -ne 0 ] && hang=y
    #     fi
    # fi

  K8SReadinessProbe.sh: |
    #!/usr/bin/env bash
    PGDATA="${PGDATA%*/}"
    status=$(gs_ctl status -D ${PGDATA})
    role="$(echo "${status}" | grep -E -i 'primary|standby' | awk -F ' ' '{print $5}' | awk -F '"' '{print $2}')"
    if [ "${role}" == "standby" ] || [ "${role}" == "primary" ]; then
      echo 0
    else
      echo 1
    fi

  K8SLivenessProbe.sh: |
    #!/usr/bin/env bash
    PGDATA="${PGDATA%*/}"
    PID="$(gs_ctl status -D ${PGDATA} | grep -Eo 'PID: ([0-9]+)' | awk -F ': ' '{print $2}')"
    if [ "${PID}" != "" ]; then
      echo 0
    else 
      echo 1
    fi
  
  k8s-initenv.sh: |
    #!/usr/bin/env bash
    # 参数默认值
    # K8S ENV or Dockerfile: PGDATA GAUSSHOME OG_PASSWORD
    PGDATA="${PGDATA%*/}"
    GAUSSHOME="${GAUSSHOME%*/}"
    export LC_ALL=en_US.UTF-8
    export LANG=en_US.UTF-8
    export TZ='Asia/Shanghai'
    ShellName="$(echo $0 | awk -F / '{print $NF}')"
    workDir=$(echo $0 | sed "s/${ShellName}//g")
    [ -z "${workDir}" ] && workDir=${PWD}
    cd ${workDir}
    workDir=${PWD}
    LogDir="${workDir}/logs"
    mkdir -p ${LogDir}
    LogFile=${LogDir}/${ShellName}.log
    UserName=$(whoami)
    HostName=$(hostname)
    ShellName="$(echo $0 | awk -F / '{print $NF}')"
    PShellName=$(echo "${ShellName}" | sed 's/^[0-9]*_//')

    # 日志输出函数
    function f_PrintLog() {
        case $(echo "$1"|tr "a-z" "A-Z") in
            INFO|INFO_) typeset _LogLevel=3;typeset _LogFlag="INFO_";;
            WARN|WARN_) typeset _LogLevel=2;typeset _LogFlag="WARN_";;
            ERROR|STOP|STOP_) typeset _LogLevel=1;typeset _LogFlag="ERROR";;
            SUCC|SUCC_|SUCCESS) typeset _LogLevel=0;typeset _LogFlag="SUCC_";;
        esac

        if [ ${_LogLevel:-3} -le ${LogLevel:-3} ]; then
            typeset LogFile="${LogFile:-"/tmp/${PShellName:-${ShellName}}.log"}"
            touch "${LogFile}" 2>/dev/null
            if [ -w "${LogFile}" ]; then
                echo "[${_LogFlag}]:$(date "+%Y%m%d.%H%M%S"):${UserName}@${HostName}:${2}"|tee -a "${LogFile}";
            else
                echo "[${_LogFlag}]:$(date "+%Y%m%d.%H%M%S"):${UserName}@${HostName}:${2}";
            fi
        fi
        # Here defined if scirpt encountered an error,then exit the script
        [ "${_LogFlag}" = "ERROR" ] && exit 55 || return 0;
    }
    # 添加 $? 判断函数。
    chkCmdOK() {
        if [ "$?" -eq 0 ]; then
            f_PrintLog "$1" "$2"
        fi
    }
    chkCmdNO() {
        if [ "$?" -ne 0 ]; then
            f_PrintLog "$1" "$2"
        fi
    }

    ########################### 开始安装 ###########################
    sudo chown -R omm:dbgrp /gaussdata
    sudo chown -R omm:dbgrp /gaussarch
    mkdir -p /gaussarch/archive
    mkdir -p /gaussarch/log/ha
    mkdir -p /gaussarch/corefile
    mkdir -p /gaussarch/log/omm/bin
    mkdir -p /gaussarch/log/omm/pg_audit
    mkdir -p /gaussarch/log/omm/pg_log
    f_PrintLog "INFO" "Check whether is new pvc"
    if [ ${NEED_INIT} != "true" ] ; then
        # configmap 挂载，解析参数。过滤注释行和空行
        cat /gauss/files/cm-mnt/postgres-cm.conf > ~/postgres-cm.conf.tmp
        sed -i 's|\t| |' ~/postgres-cm.conf.tmp
        grep -E -v '^ *#|^ *$' ~/postgres-cm.conf.tmp > ~/postgres-cm.conf
        while read line
        do
            eval $(echo ${line} | awk -F '=' '{printf("key1=%s;value1=%s",$1,$2)}')
            echo "Parsing parameter: key=${key1}, value=${value1}"
            if [ -z "${key1}" -o -z "${value1}" ]; then
                echo "Parsing parameter error, please remove the data directory for reinstallation."
                exit 1
            fi
            gs_guc set -D /gaussdata/openGauss/db1/ -c "${line}" > /dev/null
            gs_guc set -D /gaussdata/openGauss/db1/ -c "password_encryption_type=0" > /dev/null
            chkCmdNO "ERROR" 'Command failed: gs_guc set -D /gaussdata/openGauss/db1/ -c "${line}"'
        done < ~/postgres-cm.conf
        echo 'enable_numa = false' >> "${PGDATA}/mot.conf"
        
        f_PrintLog "INFO" 'Starting openGauss ...'
        gs_ctl -D ${PGDATA} -w start
        sleep 6

        # f_PrintLog "INFO" 'Start create user zabbix and backupuser ...'
        gsql -d postgres -p ${CR_DB_PORT} -c "create user dbpaasop with sysadmin monadmin password '${DBPAASOP_PASSWD}'"
        f_PrintLog "INFO" 'Stopping openGauss ...'
        gs_ctl -D ${PGDATA} -m fast -w stop
        sleep 3

        f_PrintLog "INFO" "pvc already exists, not need init"
        exit 0
    fi

    f_PrintLog "INFO" "Find or install openGauss."
    [ -f "${PGDATA}/PG_VERSION" ] && exit 0
    f_PrintLog "INFO" "File does not exist: ${PGDATA}/PG_VERSION"

    # 检测密码复杂度，8位，大小写与数字
    [[ ${#OG_PASSWORD} -ge 8 && "${OG_PASSWORD}" == *[A-Z]* && "${OG_PASSWORD}" == *[a-z]* && "${OG_PASSWORD}" == *[0-9]* ]]
    chkCmdNO "ERROR" "Password must contain at least 8 characters, and at least include the following: uppercase, lowercase, numeric."
    f_PrintLog "INFO" 'Check password success.'

    f_PrintLog "INFO" 'Start task: gs_initdb ...'
    gs_initdb --locale=C -E=UTF-8 -w "${OG_PASSWORD}" --nodename=gaussdb -D ${PGDATA}
    chkCmdNO "ERROR" 'Command failed: gs_initdb'
    f_PrintLog "INFO" 'Init process complete.'

    # configmap 挂载，解析参数。过滤注释行和空行
    cat /gauss/files/cm-mnt/postgres-cm.conf > ~/postgres-cm.conf.tmp
    sed -i 's|\t| |' ~/postgres-cm.conf.tmp
    grep -E -v '^ *#|^ *$' ~/postgres-cm.conf.tmp > ~/postgres-cm.conf
    while read line
    do
        eval $(echo ${line} | awk -F '=' '{printf("key1=%s;value1=%s",$1,$2)}')
        echo "Parsing parameter: key=${key1}, value=${value1}"
        if [ -z "${key1}" -o -z "${value1}" ]; then
            echo "Parsing parameter error, please remove the data directory for reinstallation."
            exit 1
        fi
        gs_guc set -D /gaussdata/openGauss/db1/ -c "${line}" > /dev/null
        gs_guc set -D /gaussdata/openGauss/db1/ -c "password_encryption_type=0" > /dev/null
        chkCmdNO "ERROR" 'Command failed: gs_guc set -D /gaussdata/openGauss/db1/ -c "${line}"'
    done < ~/postgres-cm.conf
    echo 'enable_numa = false' >> "${PGDATA}/mot.conf"

    f_PrintLog "INFO" 'Starting openGauss ...'
    gs_ctl -D ${PGDATA} -w start
    sleep 6

    # f_PrintLog "INFO" 'Start create user zabbix and backupuser ...'
    gsql -d postgres -p ${CR_DB_PORT} -c "create user dbpaasop with sysadmin monadmin password '${DBPAASOP_PASSWD}'"
     f_PrintLog "INFO" 'Stopping openGauss ...'
    gs_ctl -D ${PGDATA} -m fast -w stop
    sleep 3
    
  og.entrypoint.sh: |
    #!/usr/bin/env bash
    PGDATA="${PGDATA%*/}"
    export LC_ALL=en_US.UTF-8
    export LANG=en_US.UTF-8
    export TZ='Asia/Shanghai'

    ########################### 开始安装 ###########################
    mkdir -p /gaussarch/archive
    mkdir -p /gaussarch/corefile
    mkdir -p /gaussarch/log/ha
    mkdir -p /gaussarch/log/omm/bin
    mkdir -p /gaussarch/log/omm/pg_audit
    mkdir -p /gaussarch/log/omm/pg_log
    sudo chown -R omm:dbgrp /gaussdata
    sudo chown -R omm:dbgrp /gauss/files
    sudo chown -R omm:dbgrp /gaussarch
    touch /gauss/files/maintenance-bak
    touch /gauss/files/maintenance
    chmod 755 /gauss/files/*/*.sh
    ln -s /gauss/files/cm-mnt*/* /gauss/files/

    echo "Check that the database is installed."
    if [ ! -f "${PGDATA}/PG_VERSION" ]; then
        echo "Cannot find file ${PGDATA}/PG_VERSION, please check initContainer."
        exit 1
    fi

    # 查询日志pvc和存储pvc使用率，判断使用率是否达到100%
    logUsage=$(df -h "/gaussarch" | awk 'NR==2 {print $5}' | cut -d'%' -f1)
    dataUsage=$(df -h "/gaussdata/openGauss" | awk 'NR==2 {print $5}' | cut -d'%' -f1)
    echo "data pvc usage is $dataUsage%, log pvc usage is $logUsage% "
    threshold=100
    if [ "$logUsage" -ge "$threshold" ]; then
        echo "log pvc usage is $logUsage%, maybe affect the database startup"
    fi
    if [ "$dataUsage" -ge "$threshold" ]; then
        echo "data pvc usage is $dataUsage%, maybe affect the database startup"
    fi

    #处理postmaster.pid already exists场景
    if [  -f "${PGDATA}/postmaster.pid.lock" ]; then
        echo "lock file "postmaster.pid/postmaster.pid.lock" already exists, maybe affect the startup,will clean it"
        cd ${PGDATA}
        mv postmaster.pid postmaster.pid_$(date +"%Y%m%d")
        rm -rf postmaster.pid.lock
    fi
    echo 'Starting openGauss with -M pending...'
    gs_ctl -D ${PGDATA} -w start -M pending &
    [ "$?" -ne 0 ] && echo "Database startup failure." 

    # scriptrunnerPath=$(whereis scriptrunner | awk -F ':' '{print $2}')
    # if [ "$scriptrunnerPath" != '' ]; then
    #   mkdir -p /gauss/files/logs/
    #  nohup scriptrunner -c /gauss/files/script/scriptconfig-og.ini >> /gauss/files/logs/scriptrunner-og.log &
    # fi
    #通过curl到kubernetes服务判断当前Pod是否成为孤岛
    #连续30次心跳失败，则判定当前Pod成为孤岛，停止主进程使Pod重启
    count=0
    max=30
    while (( $count <= $max ))
    do
      curl -m 1 -s ${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT} >/dev/null
      result=$?
      if [ $result -eq "0" ]; then
        count=0
      else
        count=$((${count} + 1))
        current_time=$(date +"%Y-%m-%d %H:%M:%S")
        echo -e "${current_time} access ${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}, got result ${result}, count $count/$max\n" >> /gauss/files/heartbeat.log
      fi
      sleep 1s
    done
    touch /gaussarch/PID_1_FILE
    tail -s 30 -f /gaussarch/PID_1_FILE

  sidecar.entrypoint.sh: |
    #!/usr/bin/env bash
    ShellName="$(echo $0 | awk -F / '{print $NF}')"
    # 防重入函数
    funcCheckShellCount() {
        MaxRunningCount=3
        # 有时候会出现重复执行，原因不明，可能是 tini 1号进程导致
        psRes=$(ps -ef)
        echo "${psRes}"
        ShellCount=$(echo "${psRes}" | grep "${ShellName}" | grep -vE "$$|^more|^vi|^view|^grep|^tail" | awk 'END{print NR}')
        if [ ${ShellCount} -gt ${MaxRunningCount} ]; then
            echo "ERROR" "$(date "+%Y%m%d.%H%M%S"):Running script ${ShellName} ${ShellCount}"
            exit 55
        fi
    }
    funcCheckShellCount
    ############################ 开始安装 ############################
    sudo chown -R omm:dbgrp /opt/filebeat*
    sudo ln -s /gauss/files/cm-mnt/* /gauss/files/
    sudo chown -R omm:dbgrp /gauss/files
    chmod 755 /gauss/files/*/*.sh
    # scriptrunnerPath=$(whereis scriptrunner | awk -F ':' '{print $2}')
    # if [ "$scriptrunnerPath" != '' ]; then
    #   mkdir -p /gauss/files/logs/
    #   nohup scriptrunner -c /gauss/files/script/scriptconfig-sidecar.ini >> /gauss/files/logs/scriptrunner-sidecar.log &
    # fi
    
    cd /opt/filebeat
    echo "INFO" "Start filebeat ..."
    ./filebeat -e -c ./opengauss/filebeat.opengauss.yml
    
  postgres-cm.conf: |
    # initContainer 在处理时会过滤掉注释的内容，处理的逻辑是获取每一行的内容作为变量
    # 同时会判断变量是否符合 key=value 的形式，否则会报错退出
    # 不要把参数与注释放在同一行，因为不会过滤出注释
    # sed -i 's|\t| |' ~/postgres-cm.conf.tmp
    # grep -E -v '^ *#|^ *$' ~/postgres-cm.conf.tmp
    # gs_guc set -D /gaussdata/openGauss/db1/ -c "${line}"
    modify_initial_password=false
    password_encryption_type=0
    archive_mode=on
    archive_dest='/gaussdata/archive/archive_xlog'
    #max_connections=20000
    max_connections=1500
    work_mem=64MB
    # maintenance_work_mem=2GB
    # wal_buffers=1GB
    maintenance_work_mem=256MB
    wal_buffers=128MB
    cstore_buffers=16MB
    wal_level=logical
    full_page_writes=off
    wal_log_hints=off
    wal_keep_segments=1024
    xloginsert_locks=48
    advance_xlog_file_num=10
    logging_collector=on
    log_duration=on
    log_line_prefix='%m %u %d %r %p %S'
    log_checkpoints=on
    log_hostname=off
    vacuum_cost_limit=1000
    autovacuum_max_workers=10
    autovacuum_naptime=20s
    autovacuum_vacuum_cost_delay=10
    autovacuum_vacuum_scale_factor=0.05
    autovacuum_analyze_scale_factor=0.02
    autovacuum_vacuum_threshold=200
    autovacuum_analyze_threshold=200
    autovacuum_io_limits=104857600
    max_wal_senders=16
    recovery_max_workers=4
    checkpoint_segments=1024
    checkpoint_completion_target=0.8
    session_timeout=0
    enable_alarm=off
    enable_codegen=off
    lc_messages='en_US.UTF-8'
    lc_monetary='en_US.UTF-8'
    lc_numeric='en_US.UTF-8'
    lc_time='en_US.UTF-8'
    enable_wdr_snapshot=off
    audit_enabled=off
    wal_receiver_timeout=60s
    plog_merge_age=0
    update_lockwait_timeout=1min
    lockwait_timeout=1min
    max_prepared_transactions=3000
    instr_unique_sql_count=20000
    track_sql_count=on
    enable_mergejoin=on        
    port=${CR_DB_PORT}
    listen_addresses='*'
    remote_read_mode=non_authentication
    password_encryption_type=0
    # 设置备机可读
    hot_standby=on`

	YAML_SVC = `apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/app: opengauss
    app.kubernetes.io/name: ${CR_NAME}
  name: ${SVC_NAME}
  namespace: ${CR_NAMESPACE}
  ownerReferences:
  - apiVersion: ${CR_API_VERSION}
    blockOwnerDeletion: true
    controller: true
    kind: ${CR_KIND}
    name: ${CR_NAME}
    uid: ${CR_UID}
spec:
  ports:
  - nodePort: ${CR_SVC_PORT}
    port: ${CR_DB_PORT}
    protocol: TCP
    targetPort: ${CR_DB_PORT}
  selector:
    opengauss.cluster: ${CR_NAME}
    opengauss.role: ${DB_ROLE}
  type: NodePort`

	YAML_PV = `apiVersion: v1
kind: PersistentVolume
metadata:
  labels:
    app.kubernetes.io/app: opengauss
    app.kubernetes.io/name: ${CR_NAME}
    pv: ${POD_NAME}-${PV_TYPE}
  name: ${POD_NAME}-${PV_TYPE}-pv
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: ${PV_STORAGE_CAPACITY}
  hostPath:
    path: ${HOSTPATH_ROOT}/${CR_NAMESPACE}/${CR_NAME}/${POD_NAME}/${PV_TYPE}
    type: DirectoryOrCreate
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - ${PV_NODE_SELECT}
  persistentVolumeReclaimPolicy: Retain`

	YAML_PVC_HOSTPATH = `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app.kubernetes.io/app: opengauss
    app.kubernetes.io/name: ${CR_NAME}
    pvc: ${POD_NAME}-${PVC_TYPE}
  name: ${POD_NAME}-${PVC_TYPE}-pvc
  namespace: ${CR_NAMESPACE}
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: ${PVC_STORAGE_REQ}
  selector: 
      matchLabels:
          pv: ${POD_NAME}-${PVC_TYPE}`

	YAML_PVC = `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app.kubernetes.io/app: opengauss
    app.kubernetes.io/name: ${CR_NAME}
    pod: ${POD_NAME}
  name: ${POD_NAME}-${PVC_TYPE}-pvc
  namespace: ${CR_NAMESPACE}
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: ${PVC_STORAGE_REQ}
  storageClassName: ${PVC_STORAGE_CLASS}
  volumeName: ${POD_NAME}-${PVC_TYPE}`
  
	YAML_POD = `apiVersion: v1
kind: Pod
metadata:
  name: ${POD_NAME}
  annotations:
    security.alpha.kubernetes.io/unsafe-sysctls: kernel.sem = 250 6400000 1000 25600
  namespace: ${CR_NAMESPACE}
  labels:
    app.kubernetes.io/app: opengauss
    opengauss.cluster: ${CR_NAME}
  ownerReferences:
  - apiVersion: ${CR_API_VERSION}
    blockOwnerDeletion: true
    controller: true
    kind: ${CR_KIND}
    name: ${CR_NAME}
    uid: ${CR_UID}
spec:
    #allowedUnsafeSysctls:
  #- kernel.sem
  securityContext:
    sysctls:
    - name: kernel.sem
      value: "250 6400000 1000 25600"
  initContainers:
  - name: initenv
    securityContext:
      privileged: true
    image: ${POD_DB_IMG}
    imagePullPolicy: IfNotPresent
    command:
    - sh
    args:
    - -c
    - cp /gauss/files/cm-mnt/k8s-initenv.sh ~/k8s-initenv.sh && bash ~/k8s-initenv.sh
    volumeMounts:
    - name: sharedir
      mountPath: /mnt/sharedir
    - name: opengauss-cluster-scripts
      mountPath: /gauss/files/cm-mnt
    - name: pvc-data
      mountPath: /gaussdata/openGauss
      subPath: db1
    - name: pvc-log
      mountPath: /gaussarch
    env:
    - name: OG_PASSWORD
      valueFrom:
        secretKeyRef:
          name: ${CR_SECRET_NAME}
          key: INIT_PASSWD
    - name: DBPAASOP_PASSWD
      valueFrom:
        secretKeyRef:
          name: ${CR_SECRET_NAME}
          key: DBPAASOP_PASSWD
    - name: NEED_INIT
      value: ${NEED_INIT}    
  containers:
  - name: og
    securityContext:
      privileged: true
    image: ${POD_DB_IMG}
    imagePullPolicy: IfNotPresent
    command:
    - tini
    args:
    - -g
    - --
    - bash
    - -c
    - cp /gauss/files/cm-mnt/og.entrypoint.sh ~/og.entrypoint.sh && bash ~/og.entrypoint.sh
    env:
    - name: OG_PASSWORD
      valueFrom:
        secretKeyRef:
          name: ${CR_SECRET_NAME}
          key: INIT_PASSWD
    - name: MY_POD_IP
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: status.podIP
    lifecycle:
      preStop:
        exec:
          command:
          - bash
          - -c
          - gs_ctl stop -D /gaussdata/openGauss/db1
    readinessProbe:
      tcpSocket:
        port: ${CR_DB_PORT}
      failureThreshold: 10
      initialDelaySeconds: 60
      periodSeconds: ${READINESS_PROBE_PERIOD}
      successThreshold: 1
      timeoutSeconds: 1
    ports:
    - containerPort: ${CR_DB_PORT}
      name: og
      protocol: TCP
    resources:
      requests:
        cpu: ${DB_CPU_REQ}
        memory: ${DB_MEM_REQ}
      limits:
        cpu: ${DB_CPU_LMT}
        memory: ${DB_MEM_LMT}
    volumeMounts:
    - name: ogbackup
      mountPath: /gaussdata/backup
    - name: archive
      mountPath: /gaussdata/archive
    - name: pvc-data
      mountPath: /gaussdata/openGauss
      subPath: db1
    - name: pvc-log
      mountPath: /gaussarch
      subPath: log
    - name: sharedir
      mountPath: /gauss/files/sharedir
    - name: ${CLUSTER_CM_NAME}
      mountPath: /gauss/files/cm-mnt
    - name: ${SCRIPT_CM_NAME}
      mountPath: /gauss/files/script
  - name: sidecar
    securityContext:
      privileged: true
    image: ${POD_SIDECAR_IMG}
    imagePullPolicy: IfNotPresent
    command:
    - tini
    args:
    - -g
    - --
    - bash
    - -c
    - cp /gauss/files/cm-mnt/sidecar.entrypoint.sh ~/sidecar.entrypoint.sh && bash ~/sidecar.entrypoint.sh
    env:
    - name: CR_NAME
      value: ${CR_NAME}
    - name: MY_POD_IP
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: status.podIP
    resources:
      requests:
        cpu: ${SIDECAR_CPU_REQ}
        memory: ${SIDECAR_MEM_REQ}
      limits:
        cpu: ${SIDECAR_CPU_LMT}
        memory: ${SIDECAR_MEM_LMT}
    volumeMounts:
    - name: pvc-log
      mountPath: /gaussarch
      subPath: log
    - name: ogbackup
      mountPath: /gaussdata/backup
    - name: archive
      mountPath: /gaussdata/archive
    - name: sharedir
      mountPath: /gauss/files/sharedir
    - name: ${CLUSTER_CM_NAME}
      mountPath: /gauss/files/cm-mnt
    - name: ${SCRIPT_CM_NAME}
      mountPath: /gauss/files/script
    - name: ${FILEBEAT_CM_NAME}
      mountPath: /opt/filebeat/opengauss
  restartPolicy: Always
  terminationGracePeriodSeconds: ${GRACE_PERIOD}
  tolerations:
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
    tolerationSeconds: ${TOLERATION_SECOND}
  - effect: NoExecute
    key: node.kubernetes.io/unreachable
    operator: Exists
    tolerationSeconds: ${TOLERATION_SECOND}
  volumes:
  - name: sharedir
    emptyDir: {}
  - name: pvc-data
    persistentVolumeClaim:
      claimName: ${POD_NAME}-data-pvc
  - name: pvc-log
    persistentVolumeClaim:
      claimName: ${POD_NAME}-log-pvc
  - name: ${CLUSTER_CM_NAME}
    configMap:
      name: ${CLUSTER_CM_VAL}
  - name: ${SCRIPT_CM_NAME}
    configMap:
      name: ${SCRIPT_CM_VAL}
  - name: ${FILEBEAT_CM_NAME}
    configMap:
      name: ${FILEBEAT_CM_VAL}
  - name: ogbackup
    hostPath:
      path: ${CR_BACKUP_PATH}/${CR_NAMESPACE}/${CR_NAME}
      type: DirectoryOrCreate
  - name: archive
    hostPath:
      path: ${CR_ARCHIVE_PATH}/${CR_NAMESPACE}/${CR_NAME}/${POD_NAME}
      type: DirectoryOrCreate`

	YAML_SECRET = `apiVersion: v1
kind: Secret
metadata:
  labels:
    app.kubernetes.io/app: opengauss
    app.kubernetes.io/name: ${CR_NAME}
  name: ${CR_NAME}-init-sc
  namespace: ${CR_NAMESPACE}
  ownerReferences:
  - apiVersion: ${CR_API_VERSION}
    blockOwnerDeletion: true
    controller: true
    kind: ${CR_KIND}
    name: ${CR_NAME}
    uid: ${CR_UID}
type: Opaque
data:
  INIT_PASSWD: SzhTQGFkbWlu
  DBPAASOP_PASSWD: SzhTQGFkbWlu`
)

func GetParamsWithObjReference(cluster *opengaussv1.OpenGaussCluster, params map[string]string) {
	params[CR_NAME] = cluster.Name
	params[CR_NAMESPACE] = cluster.Namespace
	params[CR_API_VERSION] = cluster.APIVersion
	params[CR_KIND] = cluster.Kind
	params[CR_UID] = string(cluster.UID)
}

func GetResourceYaml(yamlstr string, params map[string]string) string {
	for k, v := range params {
		yamlstr = strings.Replace(yamlstr, "${"+k+"}", v, -1)
	}
	return yamlstr
}
