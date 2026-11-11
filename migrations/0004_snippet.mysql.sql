create table sql_snippet (
    id bigint not null auto_increment primary key,
    name varchar(80) not null,
    description varchar(500) not null default '',
    body mediumtext not null,
    driver varchar(32) not null default '',
    version bigint not null default 1,
    created_by bigint not null,
    created_at datetime(3) not null,
    updated_at datetime(3) not null,
    unique key uk_sql_snippet_owner_name (created_by, name)
) engine=innodb default charset=utf8mb4;
