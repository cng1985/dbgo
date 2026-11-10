create table app_user (
    id bigint identity(1,1) not null primary key,
    username nvarchar(64) not null,
    password_hash nvarchar(100) not null,
    display_name nvarchar(80) not null,
    role_code nvarchar(32) not null,
    disabled bit not null constraint df_app_user_disabled default 0,
    created_at datetimeoffset not null,
    updated_at datetimeoffset not null
);

create unique index uk_app_user_username on app_user (username);

create table app_session (
    id bigint identity(1,1) not null primary key,
    user_id bigint not null,
    token_hash nvarchar(64) not null,
    expires_at datetimeoffset not null,
    revoked_at datetimeoffset null,
    created_at datetimeoffset not null
);

create unique index uk_app_session_token on app_session (token_hash);
create index idx_app_session_user on app_session (user_id);

create table db_connection (
    id bigint identity(1,1) not null primary key,
    name nvarchar(80) not null,
    driver nvarchar(32) not null,
    host nvarchar(255) not null constraint df_db_connection_host default N'',
    port int not null constraint df_db_connection_port default 0,
    database_name nvarchar(128) not null constraint df_db_connection_database default N'',
    username nvarchar(128) not null constraint df_db_connection_username default N'',
    password_cipher nvarchar(max) not null constraint df_db_connection_password default N'',
    sqlite_path nvarchar(500) not null constraint df_db_connection_sqlite default N'',
    ssl_mode nvarchar(32) not null constraint df_db_connection_ssl default N'',
    remark nvarchar(500) not null constraint df_db_connection_remark default N'',
    version bigint not null constraint df_db_connection_version default 1,
    created_by bigint not null,
    created_at datetimeoffset not null,
    updated_at datetimeoffset not null
);

create unique index uk_db_connection_name on db_connection (name);

create table query_history (
    id bigint identity(1,1) not null primary key,
    connection_id bigint not null,
    connection_name nvarchar(80) not null,
    actor_id bigint not null,
    sql_text nvarchar(max) not null,
    status nvarchar(16) not null,
    row_count bigint not null constraint df_query_history_rows default 0,
    duration_ms bigint not null constraint df_query_history_duration default 0,
    error_message nvarchar(2000) not null constraint df_query_history_error default N'',
    read_only bit not null constraint df_query_history_ro default 0,
    created_at datetimeoffset not null
);

create index idx_query_history_conn on query_history (connection_id, id);
create index idx_query_history_status on query_history (status, id);

create table data_migration (
    id bigint identity(1,1) not null primary key,
    connection_id bigint not null,
    connection_name nvarchar(80) not null,
    version_no bigint not null,
    name nvarchar(120) not null,
    up_sql nvarchar(max) not null,
    down_sql nvarchar(max) not null constraint df_data_migration_down default N'',
    checksum nvarchar(64) not null,
    status nvarchar(16) not null,
    error_message nvarchar(2000) not null constraint df_data_migration_error default N'',
    applied_at datetimeoffset null,
    created_by bigint not null,
    created_at datetimeoffset not null,
    updated_at datetimeoffset not null
);

create unique index uk_data_migration_ver on data_migration (connection_id, version_no);

create table data_backup (
    id bigint identity(1,1) not null primary key,
    connection_id bigint not null,
    connection_name nvarchar(80) not null,
    driver nvarchar(32) not null,
    name nvarchar(120) not null,
    format nvarchar(16) not null,
    storage_key nvarchar(255) not null,
    size_bytes bigint not null constraint df_data_backup_size default 0,
    checksum nvarchar(64) not null constraint df_data_backup_checksum default N'',
    status nvarchar(16) not null,
    truncated bit not null constraint df_data_backup_truncated default 0,
    statement_count bigint not null constraint df_data_backup_statements default 0,
    error_message nvarchar(2000) not null constraint df_data_backup_error default N'',
    created_by bigint not null,
    created_at datetimeoffset not null
);

create index idx_data_backup_conn on data_backup (connection_id, id);
