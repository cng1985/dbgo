create table etl_task (
    id integer primary key autoincrement,
    name text not null,
    source_connection_id integer not null,
    source_connection_name text not null default '',
    source_sql text not null,
    target_connection_id integer not null,
    target_connection_name text not null default '',
    target_schema text not null default '',
    target_table text not null,
    write_mode text not null,
    mappings text not null default '[]',
    version integer not null default 1,
    created_by integer not null,
    created_at datetime not null,
    updated_at datetime not null
);

create unique index uk_etl_task_owner_name on etl_task (created_by, name);

create table etl_run (
    id integer primary key autoincrement,
    task_id integer not null,
    status text not null,
    rows_read integer not null default 0,
    rows_written integer not null default 0,
    error_message text not null default '',
    started_at datetime not null,
    finished_at datetime,
    created_by integer not null
);

create index ix_etl_run_task on etl_run (created_by, task_id, id);
