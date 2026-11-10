alter table app_user add column email text not null default '';

create table llm_profile (
    id integer primary key autoincrement,
    name text not null,
    base_url text not null,
    api_key_cipher text not null default '',
    model text not null,
    temperature real not null default 0.2,
    max_tokens integer not null default 1200,
    is_default integer not null default 0,
    remark text not null default '',
    version integer not null default 1,
    created_by integer not null,
    created_at datetime not null,
    updated_at datetime not null
);

create unique index uk_llm_profile_name on llm_profile (name);
