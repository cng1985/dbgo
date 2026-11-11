create table etl_task (
    id bigint identity(1,1) not null primary key,
    name nvarchar(80) not null,
    source_connection_id bigint not null,
    source_connection_name nvarchar(80) not null constraint df_etl_task_source_name default N'',
    source_sql nvarchar(max) not null,
    target_connection_id bigint not null,
    target_connection_name nvarchar(80) not null constraint df_etl_task_target_name default N'',
    target_schema nvarchar(128) not null constraint df_etl_task_schema default N'',
    target_table nvarchar(128) not null,
    write_mode nvarchar(16) not null,
    mappings nvarchar(max) not null constraint df_etl_task_mappings default N'[]',
    version bigint not null constraint df_etl_task_version default 1,
    created_by bigint not null,
    created_at datetimeoffset not null,
    updated_at datetimeoffset not null
);

create unique index uk_etl_task_owner_name on etl_task (created_by, name);

create table etl_run (
    id bigint identity(1,1) not null primary key,
    task_id bigint not null,
    status nvarchar(16) not null,
    rows_read bigint not null constraint df_etl_run_read default 0,
    rows_written bigint not null constraint df_etl_run_written default 0,
    error_message nvarchar(2000) not null constraint df_etl_run_error default N'',
    started_at datetimeoffset not null,
    finished_at datetimeoffset null,
    created_by bigint not null
);

create index ix_etl_run_task on etl_run (created_by, task_id, id);
