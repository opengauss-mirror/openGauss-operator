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
package utils

import (
	"sync"
)

var internalPropSet Set
var postmasterSet Set
var internalPropLock sync.Mutex
var postmasterPropLock sync.Mutex

func GetInternalProperties() Set {
	if internalPropSet.IsEmpty() {
		generateInternalProps()
	}
	return internalPropSet
}

func GetPostmasterProperties() Set {
	if postmasterSet.IsEmpty() {
		generatePostmasterProps()
	}
	return postmasterSet
}

func generateInternalProps() {
	internalPropLock.Lock()
	defer internalPropLock.Unlock()
	internalPropSet = NewSet()
	internalPropSet.Add("wal_segment_size")
	internalPropSet.Add("integer_datetimes")
	internalPropSet.Add("update_process_title")
	internalPropSet.Add("current_logic_cluster")
	internalPropSet.Add("instr_unique_sql_track_type")
	internalPropSet.Add("sql_compatibility")
	internalPropSet.Add("max_function_args")
	internalPropSet.Add("enable_adio_function")
	internalPropSet.Add("server_version")
	internalPropSet.Add("server_version_num")
	internalPropSet.Add("wal_block_size")
	internalPropSet.Add("max_identifier_length")
	internalPropSet.Add("block_size")
	internalPropSet.Add("lc_collate")
	internalPropSet.Add("lc_ctype")
	internalPropSet.Add("server_encoding")
	internalPropSet.Add("max_index_keys")
	internalPropSet.Add("segment_size")
	internalPropSet.Add("percentile")
	internalPropSet.Add("port")
}

func generatePostmasterProps() {
	postmasterPropLock.Lock()
	defer postmasterPropLock.Unlock()
	postmasterSet = NewSet()
	postmasterSet.Add("recovery_parallelism")
	postmasterSet.Add("max_files_per_process")
	postmasterSet.Add("numa_distribute_mode")
	postmasterSet.Add("transparent_encrypt_kms_region")
	postmasterSet.Add("max_cached_tuplebufs")
	postmasterSet.Add("enable_delta_store")
	postmasterSet.Add("enable_ffic_log")
	postmasterSet.Add("wal_level")
	postmasterSet.Add("recovery_max_workers")
	postmasterSet.Add("xlog_idle_flushes_before_sleep")
	postmasterSet.Add("UDFWorkerMemHardLimit")
	postmasterSet.Add("enableSeparationOfDuty")
	postmasterSet.Add("alarm_component")
	postmasterSet.Add("max_connections")
	postmasterSet.Add("shared_preload_libraries")
	postmasterSet.Add("config_file")
	postmasterSet.Add("shared_buffers")
	postmasterSet.Add("pgxc_node_name")
	postmasterSet.Add("query_log_directory")
	postmasterSet.Add("wal_buffers")
	postmasterSet.Add("perf_directory")
	postmasterSet.Add("local_bind_address")
	postmasterSet.Add("comm_sctp_port")
	postmasterSet.Add("hot_standby")
	postmasterSet.Add("max_compile_functions")
	postmasterSet.Add("transparent_encrypt_kms_url")
	postmasterSet.Add("data_replicate_buffer_size")
	postmasterSet.Add("thread_pool_attr")
	postmasterSet.Add("max_locks_per_transaction")
	postmasterSet.Add("comm_control_port")
	postmasterSet.Add("sync_config_strategy")
	postmasterSet.Add("wal_receiver_buffer_size")
	postmasterSet.Add("wal_file_init_num")
	postmasterSet.Add("comm_memory_pool")
	postmasterSet.Add("ssl_ciphers")
	postmasterSet.Add("unix_socket_directory")
	postmasterSet.Add("comm_sender_buffer_size")
	postmasterSet.Add("job_queue_processes")
	postmasterSet.Add("comm_memory_pool_percent")
	postmasterSet.Add("enable_default_cfunc_libpath")
	postmasterSet.Add("xloginsert_locks")
	postmasterSet.Add("advance_xlog_file_num")
	postmasterSet.Add("enable_memory_limit")
	postmasterSet.Add("hba_file")
	postmasterSet.Add("recovery_parse_workers")
	postmasterSet.Add("max_inner_tool_connections")
	postmasterSet.Add("cn_send_buffer_size")
	postmasterSet.Add("autovacuum_max_workers")
	postmasterSet.Add("memorypool_size")
	postmasterSet.Add("data_sync_retry")
	postmasterSet.Add("max_wal_senders")
	postmasterSet.Add("enable_stateless_pooler_reuse")
	postmasterSet.Add("ssl")
	postmasterSet.Add("max_resource_package")
	postmasterSet.Add("data_directory")
	postmasterSet.Add("max_prepared_transactions")
	postmasterSet.Add("max_pred_locks_per_transaction")
	postmasterSet.Add("ssl_crl_file")
	postmasterSet.Add("elastic_search_ip_addr")
	postmasterSet.Add("recovery_redo_workers")
	postmasterSet.Add("transparent_encrypted_string")
	postmasterSet.Add("enable_incremental_checkpoint")
	postmasterSet.Add("ssl_key_file")
	postmasterSet.Add("replication_type")
	postmasterSet.Add("audit_directory")
	postmasterSet.Add("string_hash_compatible")
	postmasterSet.Add("local_syscache_threshold")
	postmasterSet.Add("enable_orc_cache")
	postmasterSet.Add("walsender_max_send_size")
	postmasterSet.Add("ssl_ca_file")
	postmasterSet.Add("comm_max_receiver")
	postmasterSet.Add("max_concurrent_autonomous_transactions")
	postmasterSet.Add("allow_system_table_mods")
	postmasterSet.Add("remote_read_mode")
	postmasterSet.Add("enable_mix_replication")
	postmasterSet.Add("sysadmin_reserved_connections")
	postmasterSet.Add("support_extended_features")
	postmasterSet.Add("max_process_memory")
	postmasterSet.Add("udf_memory_limit")
	postmasterSet.Add("autovacuum_freeze_max_age")
	postmasterSet.Add("catchup2normal_wait_time")
	postmasterSet.Add("asp_log_directory")
	postmasterSet.Add("asp_sample_num")
	postmasterSet.Add("available_zone")
	postmasterSet.Add("bgwriter_thread_num")
	postmasterSet.Add("wal_writer_cpu")
	postmasterSet.Add("logging_collector")
	postmasterSet.Add("enable_alarm")
	postmasterSet.Add("comm_usable_memory")
	postmasterSet.Add("unix_socket_permissions")
	postmasterSet.Add("listen_addresses")
	postmasterSet.Add("unix_socket_group")
	postmasterSet.Add("memorypool_enable")
	postmasterSet.Add("comm_quota_size")
	postmasterSet.Add("lastval_supported")
	postmasterSet.Add("wal_log_hints")
	postmasterSet.Add("enable_page_lsn_check")
	postmasterSet.Add("track_activity_query_size")
	postmasterSet.Add("max_replication_slots")
	postmasterSet.Add("comm_tcp_mode")
	postmasterSet.Add("event_source")
	postmasterSet.Add("enable_nonsysadmin_execute_direct")
	postmasterSet.Add("bbox_blanklist_items")
	postmasterSet.Add("pagewriter_thread_num")
	postmasterSet.Add("enable_thread_pool")
	postmasterSet.Add("audit_data_format")
	postmasterSet.Add("use_elastic_search")
	postmasterSet.Add("ident_file")
	postmasterSet.Add("enable_global_plancache")
	postmasterSet.Add("enable_double_write")
	postmasterSet.Add("max_changes_in_memory")
	postmasterSet.Add("force_promote")
	postmasterSet.Add("ssl_cert_file")
	postmasterSet.Add("external_pid_file")
	postmasterSet.Add("cstore_buffers")
	postmasterSet.Add("mot_config_file")
}
