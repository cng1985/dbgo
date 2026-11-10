create table app_user (
    id integer primary key autoincrement,
    username text not null,
    password_hash text not null,
    display_name text not null,
    role_code text not null,
    disabled integer not null default 0,
    created_at datetime not null,
    updated_at datetime not null
);

create unique index uk_app_user_username on app_user (username);

create table app_session (
    id integer primary key autoincrement,
    user_id integer not null,
    token_hash text not null,
    expires_at datetime not null,
    revoked_at datetime,
    created_at datetime not null
);

create unique index uk_app_session_token on app_session (token_hash);
create index idx_app_session_user on app_session (user_id);

create table db_connection (
    id integer primary key autoincrement,
    name text not null,
    driver text not null,
    host text not null default '',
    port integer not null default 0,
    database_name text not null default '',
    username text not null default '',
    password_cipher text not null default '',
    sqlite_path text not null default '',
    ssl_mode text not null default '',
    remark text not null default '',
    version integer not null default 1,
    created_by integer not null,
    created_at datetime not null,
    updated_at datetime not null
);

create unique index uk_db_connection_name on db_connection (name);

create table query_history (
    id integer primary key autoincrement,
    connection_id integer not null,
    connection_name text not null,
    actor_id integer not null,
    sql_text text not null,
    status text not null,
    row_count integer not null default 0,
    duration_ms integer not null default 0,
    error_message text not null default '',
    read_only integer not null default 0,
    created_at datetime not null
);

create index idx_query_history_conn on query_history (connection_id, id);
create index idx_query_history_status on query_history (status, id);

create table data_migration (
    id integer primary key autoincrement,
    connection_id integer not null,
    connection_name text not null,
    version_no integer not null,
    name text not null,
    up_sql text not null,
    down_sql text not null default '',
    checksum text not null,
    status text not null,
    error_message text not null default '',
    applied_at datetime,
    created_by integer not null,
    created_at datetime not null,
    updated_at datetime not null
);

create unique index uk_data_migration_ver on data_migration (connection_id, version_no);

create table data_backup (
    id integer primary key autoincrement,
    connection_id integer not null,
    connection_name text not null,
    driver text not null,
    name text not null,
    format text not null,
    storage_key text not null,
    size_bytes integer not null default 0,
    checksum text not null default '',
    status text not null,
    truncated integer not null default 0,
    statement_count integer not null default 0,
    error_message text not null default '',
    created_by integer not null,
    created_at datetime not null
);

create index idx_data_backup_conn on data_backup (connection_id, id);
