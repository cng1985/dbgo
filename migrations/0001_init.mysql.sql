create table app_user (
    id bigint not null auto_increment primary key,
    username varchar(64) not null,
    password_hash varchar(100) not null,
    display_name varchar(80) not null,
    role_code varchar(32) not null,
    disabled tinyint(1) not null default 0,
    created_at datetime(3) not null,
    updated_at datetime(3) not null,
    unique key uk_app_user_username (username)
) engine=innodb default charset=utf8mb4;

create table app_session (
    id bigint not null auto_increment primary key,
    user_id bigint not null,
    token_hash char(64) not null,
    expires_at datetime(3) not null,
    revoked_at datetime(3) null,
    created_at datetime(3) not null,
    unique key uk_app_session_token (token_hash),
    key idx_app_session_user (user_id)
) engine=innodb default charset=utf8mb4;

create table db_connection (
    id bigint not null auto_increment primary key,
    name varchar(80) not null,
    driver varchar(32) not null,
    host varchar(255) not null default '',
    port int not null default 0,
    database_name varchar(128) not null default '',
    username varchar(128) not null default '',
    password_cipher text not null,
    sqlite_path varchar(500) not null default '',
    ssl_mode varchar(32) not null default '',
    remark varchar(500) not null default '',
    version bigint not null default 1,
    created_by bigint not null,
    created_at datetime(3) not null,
    updated_at datetime(3) not null,
    unique key uk_db_connection_name (name)
) engine=innodb default charset=utf8mb4;

create table query_history (
    id bigint not null auto_increment primary key,
    connection_id bigint not null,
    connection_name varchar(80) not null,
    actor_id bigint not null,
    sql_text longtext not null,
    status varchar(16) not null,
    row_count bigint not null default 0,
    duration_ms bigint not null default 0,
    error_message varchar(2000) not null default '',
    read_only tinyint(1) not null default 0,
    created_at datetime(3) not null,
    key idx_query_history_conn (connection_id, id),
    key idx_query_history_status (status, id)
) engine=innodb default charset=utf8mb4;

create table data_migration (
    id bigint not null auto_increment primary key,
    connection_id bigint not null,
    connection_name varchar(80) not null,
    version_no bigint not null,
    name varchar(120) not null,
    up_sql longtext not null,
    down_sql longtext not null,
    checksum char(64) not null,
    status varchar(16) not null,
    error_message varchar(2000) not null default '',
    applied_at datetime(3) null,
    created_by bigint not null,
    created_at datetime(3) not null,
    updated_at datetime(3) not null,
    unique key uk_data_migration_ver (connection_id, version_no)
) engine=innodb default charset=utf8mb4;

create table data_backup (
    id bigint not null auto_increment primary key,
    connection_id bigint not null,
    connection_name varchar(80) not null,
    driver varchar(32) not null,
    name varchar(120) not null,
    format varchar(16) not null,
    storage_key varchar(255) not null,
    size_bytes bigint not null default 0,
    checksum char(64) not null default '',
    status varchar(16) not null,
    truncated tinyint(1) not null default 0,
    statement_count bigint not null default 0,
    error_message varchar(2000) not null default '',
    created_by bigint not null,
    created_at datetime(3) not null,
    key idx_data_backup_conn (connection_id, id)
) engine=innodb default charset=utf8mb4;
