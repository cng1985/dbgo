create table sql_snippet (
    id integer primary key autoincrement,
    name text not null,
    description text not null default '',
    body text not null,
    driver text not null default '',
    version integer not null default 1,
    created_by integer not null,
    created_at datetime not null,
    updated_at datetime not null
);

create unique index uk_sql_snippet_owner_name on sql_snippet (created_by, name);
