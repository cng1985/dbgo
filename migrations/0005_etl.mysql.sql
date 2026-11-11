create table etl_task (
    id bigint not null auto_increment primary key,
    name varchar(80) not null,
    source_connection_id bigint not null,
    source_connection_name varchar(80) not null default '',
    source_sql mediumtext not null,
    target_connection_id bigint not null,
    target_connection_name varchar(80) not null default '',
    target_schema varchar(128) not null default '',
    target_table varchar(128) not null,
    write_mode varchar(16) not null,
    mappings mediumtext not null,
    version bigint not null default 1,
    created_by bigint not null,
    created_at datetime(3) not null,
    updated_at datetime(3) not null,
    unique key uk_etl_task_owner_name (created_by, name)
) engine=innodb default charset=utf8mb4;

create table etl_run (
    id bigint not null auto_increment primary key,
    task_id bigint not null,
    status varchar(16) not null,
    rows_read bigint not null default 0,
    rows_written bigint not null default 0,
    error_message varchar(2000) not null default '',
    started_at datetime(3) not null,
    finished_at datetime(3) null,
    created_by bigint not null,
    key ix_etl_run_task (created_by, task_id, id)
) engine=innodb default charset=utf8mb4;
