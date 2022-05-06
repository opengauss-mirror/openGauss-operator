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
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opengaussv1 "opengauss-operator/api/v1"
	"opengauss-operator/utils"
)

const (
	GAUSS_CONTAINER_NAME        = "og"
	NOHUP_CMD                   = "nohup %s"
	BASE_CMD                    = "%s %s -D /gaussdata/openGauss/db1 %s %s %s"
	BASE_ASYNC_CMD              = "%s %s -D /gaussdata/openGauss/db1 %s %s> /dev/null 2>&1 %s"
	CHECK_RESULT_CMD            = "; echo $?"
	OG_CTL_CMD                  = "gs_ctl"
	OG_CFG_CMD                  = "gs_guc"
	OG_BASEBACKUP_CMD           = "gs_basebackup"
	CFG_PARAM_RELOAD            = "reload"
	CTL_PARAM_NOTIFY            = "notify"
	CTL_PARAM_BUILD             = "build"
	CTL_PARAM_QUERY             = "query"
	CTL_PARAM_START             = "start"
	CTL_PARAM_STOP              = "stop"
	CTL_PARAM_RESTART           = "restart"
	CTL_PARAM_SWITCHOVER        = "switchover"
	CTL_PARAM_FAILOVER          = "failover"
	CTL_M_PRIMARY               = " -M primary "
	CTL_M_STANDBY               = " -M standby "
	CTL_M_PENDING               = " -M pending "
	CTL_MODE_FAST               = " -m fast"
	CTL_MODE_IMMEDIATE          = " -m immediate "
	CHECK_STATE_CMD             = `bash /gauss/files/K8SChkRepl.sh`
	ENABLE_MAINTENANCE_CMD      = "touch /gauss/files/maintenance"
	DISABLE_MAINTENANCE_CMD     = "rm -f /gauss/files/maintenance"
	GS_DELETE_CMD               = "rm -rf /gaussdata/openGauss/db1/*"
	GS_RESTORE_CMD              = "nohup bash /gauss/files/og-restore.sh -backupFile %s -dataDir /gaussdata/openGauss/db1 >> %s 2>&1 &"
	GS_CLEAR_CONNINFO_CMD       = `sed -i '/replconninfo\|application_name\|synchronous_standby_names/d' /gaussdata/openGauss/db1/postgresql.conf`
	GS_SQL_CMD                  = "gsql -d postgres -p %d -c \"%s\""
	GS_SQL_LSN_PRIMARY          = "select pg_current_xlog_location();"
	GS_SQL_LSN_STANDBY          = "select pg_last_xlog_replay_location();"
	GS_SQL_GET_PARAM            = "show %s;"
	GS_CONNECTION_STRING        = "host=%s port=%d user=%s password=%s dbname=%s sslmode=disable target_session_attrs=read-only"
	GS_DEFAULT_NAME             = "postgres"
	GS_DEFAULT_USERNAME         = "ogoperator"
	GS_SQL_REPLCONNINDEX        = `select name from pg_settings where name like 'replconninfo%' and setting not like '%localhost%' order by name asc limit 1;`
	REPL_CONN_INFO_NAME         = "replconninfo%d"
	REPL_CONN_INFO_VALUE        = "localhost=%s localport=%d localservice=%d remotehost=%s remoteport=%d remoteservice=%d"
	APPLICATION_NAME_PARAM      = "application_name"
	MOST_AVAILABLE_SYNC_PARAM   = "most_available_sync"
	SYNC_COMMIT_PARAM           = "synchronous_commit"
	DB_CONFIG_PARAM             = " -c \"%s=%s\""
	TRUST_INFO_PARAM            = " -h \"host all all %s/32 trust\""
	REMOVE_TRUST_INFO_PARAM     = " -h \"host all all %s/32 \""
	ENABLE_REMOTE_ACCESS_PARAM  = " -h \"host all all 0.0.0.0/0 sha256\""
	ENABLE_BACKUPUSER_PARAM     = " -h \"host replication backupuser 0.0.0.0/0 sha256\""
	SYNC_NAMES_PARAM_NAME       = "synchronous_standby_names"
	SYNC_NAMES_PARAM_VALUE      = "FIRST %d(%s)"
	PARAM_VALUE_OFF             = "OFF"
	PARAM_VALUE_ON              = "ON"
	SYNC_PARAM_VALUE_REMOTE     = "remote_receive"
	APP_NAME                    = "og_%s"
	MAX_XLOG_PRUNE_PARAM        = "max_size_for_xlog_prune"
	MAX_XLOG_PRUNE_VALUE        = "1048576"
	GS_BASEBACKUP_PARAM         = "--host=%s --port=%d >> %s 2>&1 &"
	RESULT_SUCCESS              = "Success to perform"
	IP_LOCALHOST                = "127.0.0.1"
	BASEBACKUP_LOG_FILE         = "/gauss/files/basebackup.log"
	RESTORE_LOG_FILE            = "/gauss/files/restore.log"
	MAX_REPL_CONN_INDEX         = 7
	BACKUP_PATH                 = "/gaussdata/backup"
	STANDBY_CHANNEL_RE          = "channel([\\s]*:\\s[\\d\\.:\\-\\>]*\\n)"
	STANDBY_CHANNEL_VALUE       = "-->([\\d\\.]+)"
	STANDBY_SYNC_PERCENT_RE     = "sync_percent([\\s]*:\\s[\\d]+%\\n)"
	STANDBY_SYNC_PERCENT_VALUE  = "([\\d]+)%"
	STANDBY_SYNC_STATE_RE       = "sync_state([\\s]*:\\s[\\w]+\\n)"
	STANDBY_SYNC_STATE_VALUE    = ":\\s([\\w]*)"
	STANDBY_SYNC_PRIORITY_RE    = "sync_priority([\\s]*:\\s[\\d]+\\n)"
	STANDBY_SYNC_PRIORITY_VALUE = "([\\d]+)"
)

var (
	StandbyChannelParser = StandbyResultParser{
		StateExp: regexp.MustCompile(STANDBY_CHANNEL_RE),
		ValueExp: regexp.MustCompile(STANDBY_CHANNEL_VALUE),
		Process: func(state *opengaussv1.SyncState, value string) {
			state.IP = value
		},
	}
	StandbySyncPercentageParser = StandbyResultParser{
		StateExp: regexp.MustCompile(STANDBY_SYNC_PERCENT_RE),
		ValueExp: regexp.MustCompile(STANDBY_SYNC_PERCENT_VALUE),
		Process: func(state *opengaussv1.SyncState, value string) {
			perc, _ := strconv.Atoi(value)
			state.Percent = perc
		},
	}
	StandbyStateParser = StandbyResultParser{
		StateExp: regexp.MustCompile(STANDBY_SYNC_STATE_RE),
		ValueExp: regexp.MustCompile(STANDBY_SYNC_STATE_VALUE),
		Process: func(state *opengaussv1.SyncState, value string) {
			state.State = value
		},
	}
	StandbyPriorityParser = StandbyResultParser{
		StateExp: regexp.MustCompile(STANDBY_SYNC_PRIORITY_RE),
		ValueExp: regexp.MustCompile(STANDBY_SYNC_PRIORITY_VALUE),
		Process: func(state *opengaussv1.SyncState, value string) {
			perc, _ := strconv.Atoi(value)
			state.Priority = perc
		},
	}
	parsers = []StandbyResultParser{
		StandbyChannelParser,
		StandbySyncPercentageParser,
		StandbyStateParser,
		StandbyPriorityParser,
	}
)

type StandbyResultParser struct {
	StateExp *regexp.Regexp
	ValueExp *regexp.Regexp
	Process  func(state *opengaussv1.SyncState, value string)
}

type IDBService interface {
	CheckDBState(pod *corev1.Pod) (utils.DBState, error)
	StartDBToStandby(pod *corev1.Pod) (utils.DBState, bool)
	StartPrimary(pod *corev1.Pod) (utils.DBState, bool)
	RestartPrimary(pod *corev1.Pod) (utils.DBState, bool)
	StartPending(pod *corev1.Pod) (utils.DBState, bool)
	StartStandby(pod *corev1.Pod) (utils.DBState, bool)
	RestartStandby(pod *corev1.Pod) (utils.DBState, bool)
	RestartPending(pod *corev1.Pod) (utils.DBState, bool)
	BuildStandBy(pod *corev1.Pod) (utils.DBState, bool)
	FindPodWithLargestLSN(pods []corev1.Pod, preference string) corev1.Pod
	ConfigDB(pod *corev1.Pod, ipArray, remoteIpArray []string, isPrimary, start bool, config map[string]string) (utils.DBState, bool, error)
	BackupDB(targetPod *corev1.Pod, sourceIP string) (utils.DBState, bool)
	StopDB(pod *corev1.Pod) (utils.DBState, bool)
	GetDBLSN(pod *corev1.Pod) (utils.LSN, error)
	SwitchPrimary(originPrimary, newPrimary *corev1.Pod) (utils.DBState, utils.DBState, error)
	AddMaintenanceFlag(pod *corev1.Pod) (utils.DBState, bool)
	RemoveMaintenanceFlag(pod *corev1.Pod) (utils.DBState, bool)
	ConfigDBProperties(pod *corev1.Pod, config map[string]string) (bool, bool, error)
	QueryStandbyState(pod *corev1.Pod) ([]opengaussv1.SyncState, error)
	IsMostAvailableEnable(pod *corev1.Pod) (bool, error)
	UpdateMostAvailable(pod *corev1.Pod, on bool) (bool, error)
	RestoreDB(pod *corev1.Pod, restoreFile string) (utils.DBState, bool)
}

type dbService struct {
	client   client.Client
	Log      logr.Logger
	executor utils.Executor
}

func NewDBService(client client.Client, logger logr.Logger) IDBService {
	service := &dbService{client: client, Log: logger}
	service.executor = utils.NewExecutor()
	return service
}

/*
启动实例为Standby
方法参数：
	pod： 当前Pod
返回值：
	实例状态
	是否成功
*/
func (d *dbService) StartDBToStandby(pod *corev1.Pod) (utils.DBState, bool) {
	crName := getCRName(pod)
	dbstate, err := d.CheckDBState(pod)
	if err != nil {
		d.Log.Error(err, fmt.Sprintf("[%s:%s]在Pod %s上查询实例状态，发生错误", pod.Namespace, crName, pod.Name))
		return dbstate, false
	}
	if dbstate.IsStandby() {
		return dbstate, true
	}
	if !dbstate.IsPending() {
		dbstate, ok := d.StartPending(pod)
		if !ok {
			d.Log.V(3).Info(fmt.Sprintf("[%s:%s]Pod %s上的实例未能启动为仲裁状态", pod.Namespace, crName, pod.Name))
			return dbstate, false
		}
	}
	if !dbstate.IsStandby() {
		dbstate, ok := d.StartStandby(pod)
		if !ok {
			d.Log.V(3).Info(fmt.Sprintf("[%s:%s]Pod %s上的实例未能启动为备实例", pod.Namespace, crName, pod.Name))
			return dbstate, false
		}
	}
	return dbstate, true
}

/*
查找LSN最大的实例
方法参数：
	pods：实例数组
	preference：偏好，如果多个实例的LSN相同，优先选择IP与preference相同的实例
返回值：
	最大的LSN值
	LSN最大的Pod
*/
func (d *dbService) FindPodWithLargestLSN(pods []corev1.Pod, preference string) corev1.Pod {
	maxLsnPod := corev1.Pod{}
	count := len(pods)
	if count > 1 {
		maxLsn := utils.LSNZero()
		for _, pod := range pods {
			lsn, err := d.GetDBLSN(&pod)
			if err != nil {
				d.Log.Error(err, fmt.Sprintf("[%s:%s]在Pod %s上查询实例LSN，发生错误", pod.Namespace, getCRName(&pod), pod.Name))
				continue
			}
			maxLsn, maxLsnPod = getMaxLSNPod(lsn, maxLsn, pod, maxLsnPod, preference)
		}
	} else if count == 1 {
		maxLsnPod = pods[0]
	}
	return maxLsnPod
}

/*
比较LSN
方法参数：
	thisLSN，thatLSN：输入的LSN值
	thisPod，thatPod： LSN对应的实例
	preference：偏好
返回值：
	LSN最大值
	LSN最大的Pod
*/
func getMaxLSNPod(thisLSN, thatLSN utils.LSN, thisPod, thatPod corev1.Pod, preference string) (utils.LSN, corev1.Pod) {
	compare := thisLSN.CompareTo(thatLSN)
	if compare > 0 {
		return thisLSN, thisPod
	} else if compare < 0 {
		return thatLSN, thatPod
	} else {
		if preference != "" {
			if thisLSN.PodIP == preference {
				return thisLSN, thisPod
			} else if thatLSN.PodIP == preference {
				return thatLSN, thatPod
			}
		}
		compare = thisLSN.CompareIP(thatLSN)
		if compare < 0 {
			return thisLSN, thisPod
		} else {
			return thatLSN, thatPod
		}
	}
}

/*
比较同城LSN
方法参数：
	thisLSN，thatLSN：输入的LSN值
	thisIP，thatIP： LSN对应的实例IP
返回值：
	LSN最大值
	LSN最大的IP
*/
func getMaxLSNIP(thisLSN, thatLSN utils.LSN, thisIP, thatIP string) (utils.LSN, string) {
	compare := thisLSN.CompareTo(thatLSN)
	if compare >= 0 {
		return thisLSN, thisIP
	} else {
		return thatLSN, thatIP
	}
}

/*
配置实例
方法参数：
	pod：当前Pod
	ipArray：本地IP数组
	remoteIpArray：同城IP数组
	isPrimary：是否设置为Primary
	start：是否启动服务
	config：数据库配置参数
返回值：
	实例状态
	是否成功
	错误信息
方法逻辑：
	将本地IP和同城IP配置到连接信息中，并设置application_name, synchronous_standby_names等参数
	将本地IP和同城IP配置到白名单
	设置数据库配置
	如果参数中有要求重启的，重启服务
	如果没有要求重启的，但当前数据库状态与期望不符，则重启为期望状态
*/
func (d *dbService) ConfigDB(pod *corev1.Pod, ipArray, remoteIpArray []string, isPrimary, start bool, config map[string]string) (utils.DBState, bool, error) {
	crName := getCRName(pod)
	dbstate, err := d.CheckDBState(pod)
	if err != nil {
		return dbstate, false, err
	}
	configured, e := d.processOgConfig(pod, ipArray, remoteIpArray)
	if !configured {
		d.Log.Error(e, fmt.Sprintf("[%s:%s]在Pod %s上修改opengaussql.conf，发生错误", pod.Namespace, crName, pod.Name))
		return dbstate, configured, e
	}
	configured, e = d.processWhileList(pod, ipArray, remoteIpArray)
	if !configured {
		d.Log.Error(e, fmt.Sprintf("[%s:%s]在Pod %s上修改pg_hba.conf，发生错误", pod.Namespace, crName, pod.Name))
		return dbstate, configured, e
	}
	restartRequired := false
	if len(config) > 0 {
		configured, restartRequired, e = d.ConfigDBProperties(pod, config)
		if !configured {
			d.Log.Error(e, fmt.Sprintf("[%s:%s]在Pod %s上设置数据库参数，发生错误", pod.Namespace, crName, pod.Name))
			return dbstate, configured, e
		}
	}
	dbstate, err = d.CheckDBState(pod)
	if err != nil {
		return dbstate, false, err
	}
	if !start && !restartRequired {
		return dbstate, configured, nil
	}
	if restartRequired {
		if isPrimary || dbstate.IsPrimary() {
			dbstate, configured = d.RestartPrimary(pod)
		} else {
			dbstate, configured = d.RestartStandby(pod)
		}
	} else if dbstate.IsPending() {
		if isPrimary {
			dbstate, configured = d.RestartPrimary(pod)
		} else {
			dbstate, configured = d.RestartStandby(pod)
		}
	} else if dbstate.IsStandby() && isPrimary {
		dbstate, configured = d.RestartPrimary(pod)
	}
	if !configured {
		d.Log.V(3).Info(fmt.Sprintf("[%s:%s]在Pod %s上启动实例失败", pod.Namespace, crName, pod.Name))
		return dbstate, configured, nil
	}
	if dbstate.IsInMaintenance() {
		dbstate, configured = d.RemoveMaintenanceFlag(pod)
	}
	return dbstate, configured, nil
}

/*
恢复数据
方法参数：
	pod：当前Pod
	restoreFile：数据文件名称
	ipArray：应配置的IP数组
	remoteIpArray：应配置的远程IP数组
	config：数据库配置参数
方法逻辑：
	检查数据文件是否可以访问到
	停止数据库进程
	解压数据文件到数据目录，为所有自定义表空间设置好软链接
	以pending模式启动
	重新配置数据库，以主库身份启动
*/
func (d *dbService) RestoreDB(pod *corev1.Pod, restoreFile string) (utils.DBState, bool) {
	crName := getCRName(pod)
	restoreFilePath := BACKUP_PATH + string(os.PathSeparator) + restoreFile
	dbstate := utils.InitDBState()
	result := false
	if state, ok := d.StopDB(pod); !ok {
		return state, false
	} else {
		dbstate = state
	}
	command := fmt.Sprintf(GS_RESTORE_CMD, restoreFilePath, RESTORE_LOG_FILE)
	if _, err := d.executeCommand(pod.Namespace, pod.Name, command); err != nil {
		return dbstate, result
	}
	retryCount := 0
	for {
		state, e := d.CheckDBState(pod)
		if e != nil {
			d.Log.Error(e, fmt.Sprintf("[%s:%s]获取Pod %s的实例状态，发生错误", pod.Namespace, crName, pod.Name))
			return state, false
		} else {
			if state.IsRestoreComplete() {
				dbstate = state
				result = true
				break
			} else if (!state.RestoreStarted() || state.IsRestoreFailed()) && retryCount > utils.RETRY_LIMIT {
				d.Log.V(3).Info(fmt.Sprintf("[%s:%s]从文件%s向实例%s恢复数据失败", pod.Namespace, crName, pod.Status.PodIP, restoreFile))
				return state, false
			}
			d.Log.V(1).Info(fmt.Sprintf("[%s:%s]在实例%s上恢复数据，已执行%d秒", pod.Namespace, crName, pod.Status.PodIP, utils.RETRY_INTERVAL*retryCount))
		}
		retryCount++
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
	d.executeCommand(pod.Namespace, pod.Name, GS_CLEAR_CONNINFO_CMD)
	return dbstate, true
}

/*
复制数据
方法参数：
	pod：当前Pod
	sourceIP：数据源IP
返回值：
	实例状态
	是否成功
方法逻辑：
	通过调用basebackup，执行数据复制
	不断检查输出日志，直至成功，或显示失败（由于脚本不完备，可能在启动时误报错误，故只在检查超过固定次数仍然报告失败时判定为复制失败）
*/
func (d *dbService) BackupDB(pod *corev1.Pod, sourceIP string) (utils.DBState, bool) {
	crName := getCRName(pod)
	d.StopDB(pod)
	dbstate := utils.InitDBState()
	result := false
	_, err := d.executeCommand(pod.Namespace, pod.Name, GS_DELETE_CMD)
	if err != nil {
		return dbstate, result
	}
	params := fmt.Sprintf(GS_BASEBACKUP_PARAM, sourceIP, getDBPort(pod), BASEBACKUP_LOG_FILE)
	_, err = d.basebackup(pod.Namespace, pod.Name, params)
	if err != nil {
		return dbstate, result
	}
	retryCount := 0
	for {
		state, e := d.CheckDBState(pod)
		if e != nil {
			d.Log.Error(e, fmt.Sprintf("[%s:%s]获取Pod %s的实例状态，发生错误", pod.Namespace, crName, pod.Name))
			return state, false
		} else {
			if state.IsBackupComplete() {
				dbstate = state
				result = true
				break
			} else if (!state.BackupStarted() || state.IsBackupFailed()) && retryCount > utils.RETRY_LIMIT {
				d.Log.V(3).Info(fmt.Sprintf("[%s:%s]从%s向%s复制数据失败", pod.Namespace, crName, sourceIP, pod.Status.PodIP))
				return state, false
			}
			d.Log.V(1).Info(fmt.Sprintf("[%s:%s]从%s向%s数据复制中，已执行%d秒", pod.Namespace, crName, sourceIP, pod.Status.PodIP, utils.RETRY_INTERVAL*retryCount))
		}
		retryCount++
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
	if result {
		d.executeCommand(pod.Namespace, pod.Name, GS_CLEAR_CONNINFO_CMD)
	}
	return dbstate, result
}

/*
获取实例状态
方法参数：
	pod：当前Pod
返回值：
	实例状态
	错误信息
状态文本通过挂载ConfigMap并执行K8SChkRepl.sh获取，包括
    chkprocess        表示opengauss进程是否存在，1是0否
    chkconn           表示opengauss进程是否存在，1是0否
    maintenance       是否处于维护模式，1是0否
    pending           是否pending状态，1是0否
    primary           是否为主节点，1是0否
    standby           是否为备节点，1是0否
    standalone        是否为单节点，1是0否
    hang              是否hang住，1是0否
    chkrepl           主从状态是否正常，1是0否
    buildstatus       build状态：0 成功；1 失败；2 正在build
    basebackup        备份状态：0 没有备份日志文件；1 成功；2 失败；3 备份中
    detailinfo        文本信息
*/
func (d *dbService) CheckDBState(pod *corev1.Pod) (utils.DBState, error) {
	crName := getCRName(pod)
	status := utils.DBState{}
	if pod.Status.Phase != corev1.PodRunning {
		return status, fmt.Errorf("[%s:%s]Pod %s未处于运行阶段", pod.Namespace, crName, pod.Name)
	}
	result, err := d.executeCommand(pod.Namespace, pod.Name, CHECK_STATE_CMD)
	result = strings.Replace(result, "\n", "", -1)
	if err = json.Unmarshal([]byte(result), &status); err != nil {
		d.Log.Error(err, fmt.Sprintf("[%s:%s]解析位于Pod %s上的数据库状态查询结果，发生错误，查询结果： %s", pod.Namespace, crName, pod.Name, result))
		return status, err
	}
	d.Log.V(1).Info(fmt.Sprintf("[%s:%s]位于Pod %s上的数据库状态：%s", pod.Namespace, crName, pod.Name, status.PrintableString()))
	return status, nil
}

/*
查询本地实例LSN
*/
func (d *dbService) GetDBLSN(pod *corev1.Pod) (utils.LSN, error) {
	state, e := d.CheckDBState(pod)
	if e != nil {
		return utils.LSNZero(), e
	}
	query := GS_SQL_LSN_STANDBY
	if state.IsPrimary() {
		query = GS_SQL_LSN_PRIMARY
	}
	command := fmt.Sprintf(GS_SQL_CMD, getDBPort(pod), query)
	LSNStr, err := d.executeCommand(pod.Namespace, pod.Name, command)
	if err != nil {
		return utils.LSNZero(), err
	}
	return utils.ParseLSN(pod.Name, pod.Status.PodIP, LSNStr), nil
}

/*

在Pod中添加维护标记文件，用于停止数据库进程时防止Pod重启
*/
func (d *dbService) AddMaintenanceFlag(pod *corev1.Pod) (utils.DBState, bool) {
	_, err := d.executeCommand(pod.Namespace, pod.Name, ENABLE_MAINTENANCE_CMD)
	if err != nil {
		return utils.InitDBState(), false
	}
	state := d.waitState(pod, waitMaintenanceFunc)
	return state, state.IsInMaintenance()
}

/*
移除维护标记文件
*/
func (d *dbService) RemoveMaintenanceFlag(pod *corev1.Pod) (utils.DBState, bool) {
	_, err := d.executeCommand(pod.Namespace, pod.Name, DISABLE_MAINTENANCE_CMD)
	if err != nil {
		return utils.InitDBState(), false
	}
	state := d.waitState(pod, waitNotMaintenanceFunc)
	return state, !state.IsInMaintenance()
}

/*
停止实例
*/
func (d *dbService) StopDB(pod *corev1.Pod) (utils.DBState, bool) {
	dbstate, ok := d.AddMaintenanceFlag(pod)
	if !ok {
		return dbstate, ok
	}
	if dbstate.IsProcessExist() {
		_, err := d.stop(pod.Namespace, pod.Name)
		if err != nil {
			return dbstate, false
		}
	}
	state := d.waitState(pod, waitDBStopFunc)
	return state, !state.IsProcessExist()
}

/*
主从切换
方法参数：
	originPrimary：原主Pod
	newPrimary：新主Pod
返回值：
	原主实例状态
	新主实例状态
	错误信息
*/
func (d *dbService) SwitchPrimary(originPrimary, newPrimary *corev1.Pod) (utils.DBState, utils.DBState, error) {
	//重启原主，由于Labe已删除，重启将断掉现有连接，避免持续的数据写入
	_, ok := d.RestartPrimary(originPrimary)
	if !ok {
		return utils.InitDBState(), utils.InitDBState(), fmt.Errorf("[%s:%s]主节点%s重启失败", originPrimary.Namespace, getCRName(originPrimary), originPrimary.Status.PodIP)
	}
	_, err := d.switchover(newPrimary.Namespace, newPrimary.Name)
	if err != nil {
		return utils.InitDBState(), utils.InitDBState(), err
	}
	newPrimaryState, primaryOK := d.waitPrimary(newPrimary)

	originPrimaryState, standbyOK := d.waitStandby(originPrimary)
	if primaryOK && standbyOK {
		return originPrimaryState, newPrimaryState, nil
	} else {
		return originPrimaryState, newPrimaryState, fmt.Errorf("[%s:%s]主从切换失败", newPrimary.Namespace, getCRName(newPrimary))
	}
}

func (d *dbService) StartPrimary(pod *corev1.Pod) (utils.DBState, bool) {
	_, err := d.notify(pod.Namespace, pod.Name, CTL_M_PRIMARY)
	if err != nil {
		return utils.InitDBState(), false
	}
	return d.waitPrimary(pod)
}

func (d *dbService) RestartPrimary(pod *corev1.Pod) (utils.DBState, bool) {
	_, err := d.restart(pod.Namespace, pod.Name, CTL_M_PRIMARY, CTL_MODE_IMMEDIATE)
	if err != nil {
		return utils.InitDBState(), false
	}
	return d.waitPrimary(pod)
}

func (d *dbService) StartPending(pod *corev1.Pod) (utils.DBState, bool) {
	_, err := d.start(pod.Namespace, pod.Name, CTL_M_PENDING)
	if err != nil {
		return utils.InitDBState(), false
	}
	return d.waitPending(pod)
}

func (d *dbService) StartStandby(pod *corev1.Pod) (utils.DBState, bool) {
	_, err := d.notify(pod.Namespace, pod.Name, CTL_M_STANDBY)
	if err != nil {
		return utils.InitDBState(), false
	}
	return d.waitStandby(pod)
}

func (d *dbService) RestartStandby(pod *corev1.Pod) (utils.DBState, bool) {
	_, err := d.restart(pod.Namespace, pod.Name, CTL_M_STANDBY, CTL_MODE_IMMEDIATE)
	if err != nil {
		return utils.InitDBState(), false
	}
	return d.waitStandby(pod)
}

func (d *dbService) RestartPending(pod *corev1.Pod) (utils.DBState, bool) {
	_, err := d.restart(pod.Namespace, pod.Name, CTL_M_PENDING, CTL_MODE_IMMEDIATE)
	if err != nil {
		return utils.InitDBState(), false
	}
	return d.waitPending(pod)
}

func (d *dbService) BuildStandBy(pod *corev1.Pod) (utils.DBState, bool) {
	_, err := d.build(pod.Namespace, pod.Name)
	if err != nil {
		return utils.InitDBState(), false
	}
	f := func(state utils.DBState) bool {
		return state.IsStandby() && state.IsBuildComplete()
	}
	state := d.waitState(pod, f)
	return state, state.IsStandby() && state.IsBuildComplete()
}

func (d *dbService) waitState(pod *corev1.Pod, function StateCheckFunc) utils.DBState {
	errCount := 0
	dbstate := utils.InitDBState()
	crName := getCRName(pod)
	for {
		state, err := d.CheckDBState(pod)
		if err == nil && function(state) {
			dbstate = state
			break
		}
		if err != nil {
			d.Log.V(1).Info(fmt.Sprintf("[%s:%s]查询Pod %s上的数据库状态，发生错误，详情：%s，进行第%d次重试", pod.Namespace, crName, pod.Name, err.Error(), errCount))
		} else {
			d.Log.V(1).Info(fmt.Sprintf("[%s:%s]Pod %s上的数据库状态未达到期望，进行第%d次重试", pod.Namespace, crName, pod.Name, errCount))
		}
		if errCount == utils.RETRY_LIMIT {
			d.Log.V(3).Info(fmt.Sprintf("[%s:%s]等待Pod %s上的数据库状态变更超时", pod.Namespace, crName, pod.Name))
			break
		}
		errCount++
		time.Sleep(time.Second * utils.RETRY_INTERVAL)
	}
	return dbstate
}

/*
配置数据库参数
方法参数：
	pod：当前Pod
	config：数据库参数
返回值：
	是否配置成功
	是否需要重启数据库进程
	错误信息
*/
func (d *dbService) ConfigDBProperties(pod *corev1.Pod, config map[string]string) (bool, bool, error) {
	restartRequired := false
	params := ""
	restartProps := utils.GetPostmasterProperties()
	for key, value := range config {
		params += generateDBConfigPropParam(key, value, false)
		if restartProps.Contains(key) {
			restartRequired = true
		}
	}
	result, err := d.reload(pod.Namespace, pod.Name, params)
	success := isExecuteSucceeded(result)
	return success, restartRequired, err
}
func (d *dbService) QueryStandbyState(pod *corev1.Pod) ([]opengaussv1.SyncState, error) {
	states := make([]opengaussv1.SyncState, 0)
	if dbstate, e := d.CheckDBState(pod); e != nil {
		return states, e
	} else if !dbstate.IsPrimary() {
		return states, fmt.Errorf("实例%s不是primary", pod.Status.PodIP)
	}
	output, err := d.query(pod.Namespace, pod.Name)
	if err != nil {
		return states, err
	}
	for _, parser := range parsers {
		findLines := parser.StateExp.FindAllStringSubmatch(output, -1)
		if len(findLines) > 0 {
			if len(states) == 0 {
				for i := 0; i < len(findLines); i++ {
					states = append(states, opengaussv1.SyncState{})
				}
			}
			for index, line := range findLines {
				findValue := parser.ValueExp.FindStringSubmatch(line[1])
				if len(findValue) > 0 {
					value := findValue[1]
					parser.Process(&states[index], value)
				}
			}
		}
	}
	if len(states) > 0 {
		sort.Slice(states, func(i, j int) bool {
			return states[i].Priority < states[j].Priority
		})
	}
	return states, nil
}
func (d *dbService) IsMostAvailableEnable(pod *corev1.Pod) (bool, error) {
	query := fmt.Sprintf(GS_SQL_GET_PARAM, MOST_AVAILABLE_SYNC_PARAM)
	command := fmt.Sprintf(GS_SQL_CMD, getDBPort(pod), query)
	queryResult, err := d.executeCommand(pod.Namespace, pod.Name, command)
	if err != nil {
		return false, err
	}
	on := strings.Contains(queryResult, strings.ToLower(PARAM_VALUE_ON))
	return on, nil
}
func (d *dbService) UpdateMostAvailable(pod *corev1.Pod, on bool) (bool, error) {
	flag, err := d.IsMostAvailableEnable(pod)
	if err != nil {
		return false, err
	}
	if on == flag {
		return false, nil
	}
	value := PARAM_VALUE_OFF
	if on {
		value = PARAM_VALUE_ON
	}
	params := generateDBConfigPropParam(MOST_AVAILABLE_SYNC_PARAM, value, true)
	result, err := d.reload(pod.Namespace, pod.Name, params)
	success := isExecuteSucceeded(result)
	return success, err
}

/*
在postgres.conf中配置连接信息
方法参数：
	pod：当前Pod
	ipArr：本地IP数组
	remoteIpArray：同城IP数组
返回值：
	是否配置成功
	错误信息
方法逻辑：
	如果本地IP仅有一个，同城IP为空，则将localhost配置为唯一连接信息，使本地单实例表现为Primary而不是standalone
	如果本地IP超过一个，或有同城IP，opengauss能够支持的replconninfo一共有7个
		将所有IP配置为replconninfo，并将其余replconninfo置为空
		设置application_name
		设置synchronous_standby_names为ANY N(本地和同城的application_name)，N为本地实例数目
		N的设置确保本地的全部Standby和同城的至少一个Standby为同步Standby
		设置synchronous_commit的值，如果仅有本地单节点设置为OFF，否则为remote_receive
*/
func (d *dbService) processOgConfig(pod *corev1.Pod, ipArr, remoteIpArray []string) (bool, error) {
	params := ""
	IP := pod.Status.PodIP
	port := getDBPort(pod)
	// for single instance, add itself so that og 2.0 can start as primary
	if len(ipArr) == 1 && IP == ipArr[0] && len(remoteIpArray) == 0 {
		params += generateReplconninfo(IP_LOCALHOST, IP_LOCALHOST, 1, port)
		params += generateDBConfigPropParam(MAX_XLOG_PRUNE_PARAM, MAX_XLOG_PRUNE_VALUE, false)
	} else {
		index := 1
		for i := 0; i < len(ipArr); i++ {
			if IP != ipArr[i] {
				params += generateReplconninfo(IP, ipArr[i], index, port)
				index++
			}
		}
		for i := 0; i < len(remoteIpArray); i++ {
			params += generateReplconninfo(IP, remoteIpArray[i], index, port)
			index++
		}
		if len(ipArr) < 7 {
			for i := index; i <= 7; i++ {
				params += generateEmptyReplConnInfo(i)
			}
		}
	}

	params += generateAppInfo(IP)

	params += generateSyncNames(IP, ipArr, remoteIpArray)
	single := len(ipArr) == 1 && len(remoteIpArray) == 0
	params += generateSyncCommitParam(single)

	result, err := d.reload(pod.Namespace, pod.Name, params)
	success := isExecuteSucceeded(result)
	return success, err
}

/*
设置白名单
*/
func (d *dbService) processWhileList(pod *corev1.Pod, ipArr, remoteIpArray []string) (bool, error) {
	params := ""
	for _, localIp := range ipArr {
		params += generateTrustInfo(localIp)
	}
	for _, remoteIp := range remoteIpArray {
		params += generateTrustInfo(remoteIp)
	}
	params += ENABLE_REMOTE_ACCESS_PARAM
	params += ENABLE_BACKUPUSER_PARAM
	result, err := d.reload(pod.Namespace, pod.Name, params)
	success := isExecuteSucceeded(result)
	return success, err
}

func generateTrustInfo(ip string) string {
	return fmt.Sprintf(TRUST_INFO_PARAM, ip)
}

func generateRemoveTrustInfo(ip string) string {
	return fmt.Sprintf(REMOVE_TRUST_INFO_PARAM, ip)
}

func generateReplconninfo(localIp, remoteIp string, index int, dbPort int32) string {
	localPort := dbPort + 2
	localServicePort := dbPort
	remotePort := dbPort + 2
	remoteServicePort := dbPort
	connVal := fmt.Sprintf(REPL_CONN_INFO_VALUE, localIp, localPort, localServicePort, remoteIp, remotePort, remoteServicePort)
	connName := fmt.Sprintf(REPL_CONN_INFO_NAME, index)
	return generateDBConfigPropParam(connName, connVal, true)
}

func generateEmptyReplConnInfo(index int) string {
	return generateDBConfigPropParam(fmt.Sprintf(REPL_CONN_INFO_NAME, index), "", true)
}

func generateAppInfo(ip string) string {
	return generateDBConfigPropParam(APPLICATION_NAME_PARAM, generateApplicationName(ip), true)
}

func generateApplicationName(ip string) string {
	return fmt.Sprintf(APP_NAME, strings.Replace(ip, ".", "_", -1))
}
func generateSyncCommitParam(single bool) string {
	value := SYNC_PARAM_VALUE_REMOTE
	if single {
		value = PARAM_VALUE_OFF
	}
	return generateDBConfigPropParam(SYNC_COMMIT_PARAM, value, false)
}

/*
生成synchronous_standby_names与most_available_sync参数
synchronous_standby_names的取值规则：
	如果ipArr仅有一个元素，且remoteIpArray为空，则不设置
	如果remoteIpArray为空
		如果ipArr有两个元素（即除自身外还有一个IP），且remoteIpArray为空，则设置为除自身外的IP
		如果ipArr有超过两个元素，则计算quorum为ipArr的数量除以2并向上取整，参数设置为ANY quorum(ipArr)
	如果remoteIpArray不为空
		同步数量设置为ipArr的数量，即本地全部为同步从，远程确保至少一个同步从，参数设置为
		FIRST len(ipArr) (ipArr+remoteIpArray)
*/
func generateSyncNames(ip string, ipArr, remoteIpArray []string) string {
	syncValue := ""
	mostAvailableValue := PARAM_VALUE_ON
	if len(remoteIpArray) > 0 || len(ipArr) > 1 {
		var buf strings.Builder
		for _, localIp := range ipArr {
			if localIp == ip {
				continue
			}
			appName := generateApplicationName(localIp)
			if buf.Len() > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(appName)
		}
		if len(remoteIpArray) > 0 {
			for _, remoteIp := range remoteIpArray {
				appName := generateApplicationName(remoteIp)
				if buf.Len() > 0 {
					buf.WriteString(",")
				}
				buf.WriteString(appName)
			}
		}

		available := utils.CalculateSyncCount(len(ipArr), len(remoteIpArray))
		if available == 0 {
			syncValue = buf.String()
		} else {
			syncValue = fmt.Sprintf(SYNC_NAMES_PARAM_VALUE, available, buf.String())
		}
		mostAvailableValue = PARAM_VALUE_OFF
	}

	syncNames := generateDBConfigPropParam(SYNC_NAMES_PARAM_NAME, syncValue, true)
	mostAvailable := generateDBConfigPropParam(MOST_AVAILABLE_SYNC_PARAM, mostAvailableValue, true)
	return syncNames + mostAvailable
}

func (d *dbService) executeCommand(namespace, name string, command string) (string, error) {
	d.Log.V(1).Info(fmt.Sprintf("[%s:%s]执行命令：%s", namespace, name, command))
	stdOut, stdErr, err := d.executor.Select(namespace, name, GAUSS_CONTAINER_NAME).Exec(command)
	if err != nil {
		d.Log.Error(err, fmt.Sprintf("[Executor]执行命令\"%s\"失败, \n结果：%s\n错误信息：%s", command, stdOut, stdErr))
	}
	return stdOut, err
}
func (d *dbService) start(namespace, name, serverMode string) (string, error) {
	return d.executeOgCommand(namespace, name, OG_CTL_CMD, CTL_PARAM_START, serverMode, "", true, false, false)
}
func (d *dbService) notify(namespace, name, serverMode string) (string, error) {
	return d.executeOgCommand(namespace, name, OG_CTL_CMD, CTL_PARAM_NOTIFY, serverMode, "", true, false, false)
}
func (d *dbService) restart(namespace, name, serverMode, option string) (string, error) {
	return d.executeOgCommand(namespace, name, OG_CTL_CMD, CTL_PARAM_RESTART, serverMode, option, true, false, false)
}
func (d *dbService) stop(namespace, name string) (string, error) {
	return d.executeOgCommand(namespace, name, OG_CTL_CMD, CTL_PARAM_STOP, "", CTL_MODE_IMMEDIATE, true, false, false)
}
func (d *dbService) basebackup(namespace, name, parameter string) (string, error) {
	return d.executeOgCommand(namespace, name, OG_BASEBACKUP_CMD, "", parameter, "", false, true, false)
}
func (d *dbService) switchover(namespace, name string) (string, error) {
	return d.executeOgCommand(namespace, name, OG_CTL_CMD, CTL_PARAM_SWITCHOVER, "", CTL_MODE_FAST, true, false, false)
}
func (d *dbService) build(namespace, name string) (string, error) {
	return d.executeOgCommand(namespace, name, OG_CTL_CMD, CTL_PARAM_BUILD, CTL_M_STANDBY, "", true, false, false)
}
func (d *dbService) reload(namespace, name, parameter string) (string, error) {
	return d.executeOgCommand(namespace, name, OG_CFG_CMD, CFG_PARAM_RELOAD, parameter, "", true, false, true)
}
func (d *dbService) query(namespace, name string) (string, error) {
	return d.executeOgCommand(namespace, name, OG_CTL_CMD, CTL_PARAM_QUERY, "", "", false, false, false)
}
func generateCommand(baseCmd, exec, action, params, option, checkResult string) string {
	return strings.TrimSpace(fmt.Sprintf(baseCmd, exec, action, params, option, checkResult))
}
func (d *dbService) executeOgCommand(namespace, name, exec, action, parameter, options string, async, nohup, checkResult bool) (string, error) {
	baseCommand := BASE_CMD
	if async {
		baseCommand = BASE_ASYNC_CMD
	}
	checkResultCommand := ""
	if checkResult {
		checkResultCommand = CHECK_RESULT_CMD
	}
	command := generateCommand(baseCommand, exec, action, parameter, options, checkResultCommand)
	if nohup {
		command = fmt.Sprintf(NOHUP_CMD, command)
	}
	d.Log.V(1).Info(fmt.Sprintf("[%s:%s]执行命令：%s", namespace, name, command))
	stdOut, stdErr, err := d.executor.Select(namespace, name, GAUSS_CONTAINER_NAME).Exec(command)
	if err != nil {
		d.Log.Error(err, fmt.Sprintf("[Executor]执行命令\"%s\"失败, \n结果：%s\n错误信息：%s", command, stdOut, stdErr))
	}
	return stdOut, err
}
func generateDBConfigPropParam(name, value string, quote bool) string {
	paramValue := value
	if quote {
		paramValue = fmt.Sprintf("'%s'", value)
	}
	return fmt.Sprintf(DB_CONFIG_PARAM, name, paramValue)
}
func isExecuteSucceeded(stdout string) bool {
	return stdout == "0"
}
func (d *dbService) waitPending(pod *corev1.Pod) (utils.DBState, bool) {
	state := d.waitState(pod, waitPendingFunc)
	return state, state.IsPending()
}
func (d *dbService) waitPrimary(pod *corev1.Pod) (utils.DBState, bool) {
	state := d.waitState(pod, waitPrimaryFunc)
	return state, state.IsPrimary()
}
func (d *dbService) waitStandby(pod *corev1.Pod) (utils.DBState, bool) {
	state := d.waitState(pod, waitStandbyFunc)
	return state, state.IsStandby()
}
func getDBPort(pod *corev1.Pod) int32 {
	dbContainer := pod.Spec.Containers[0]
	return dbContainer.Ports[0].ContainerPort
}
func getCRName(pod *corev1.Pod) string {
	return pod.Labels[OPENGAUSS_CLUSTER_KEY]
}

type StateCheckFunc func(state utils.DBState) bool

func waitPrimaryFunc(state utils.DBState) bool {
	return state.IsPrimary()
}
func waitStandbyFunc(state utils.DBState) bool {
	return state.IsStandby()
}
func waitPendingFunc(state utils.DBState) bool {
	return state.IsPending()
}
func waitMaintenanceFunc(state utils.DBState) bool {
	return state.IsInMaintenance()
}
func waitNotMaintenanceFunc(state utils.DBState) bool {
	return !state.IsInMaintenance()
}
func waitDBStopFunc(state utils.DBState) bool {
	return !state.IsProcessExist()
}
