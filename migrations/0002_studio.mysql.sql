alter table app_user add column email varchar(120) not null default '';

create table llm_profile (
    id bigint not null auto_increment primary key,
    name varchar(80) not null,
    base_url varchar(500) not null,
    api_key_cipher text not null,
    model varchar(120) not null,
    temperature double not null default 0.2,
    max_tokens int not null default 1200,
    is_default tinyint(1) not null default 0,
    remark varchar(500) not null default '',
    version bigint not null default 1,
    created_by bigint not null,
    created_at datetime(3) not null,
    updated_at datetime(3) not null,
    unique key uk_llm_profile_name (name)
) engine=innodb default charset=utf8mb4;
