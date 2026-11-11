create table sql_snippet (
    id bigint identity(1,1) not null primary key,
    name nvarchar(80) not null,
    description nvarchar(500) not null constraint df_sql_snippet_description default N'',
    body nvarchar(max) not null,
    driver nvarchar(32) not null constraint df_sql_snippet_driver default N'',
    version bigint not null constraint df_sql_snippet_version default 1,
    created_by bigint not null,
    created_at datetimeoffset not null,
    updated_at datetimeoffset not null
);

create unique index uk_sql_snippet_owner_name on sql_snippet (created_by, name);
