alter table app_user add email nvarchar(120) not null constraint df_app_user_email default N'';

create table llm_profile (
    id bigint identity(1,1) not null primary key,
    name nvarchar(80) not null,
    base_url nvarchar(500) not null,
    api_key_cipher nvarchar(max) not null constraint df_llm_profile_key default N'',
    model nvarchar(120) not null,
    temperature float not null constraint df_llm_profile_temp default 0.2,
    max_tokens int not null constraint df_llm_profile_tokens default 1200,
    is_default bit not null constraint df_llm_profile_default default 0,
    remark nvarchar(500) not null constraint df_llm_profile_remark default N'',
    version bigint not null constraint df_llm_profile_version default 1,
    created_by bigint not null,
    created_at datetimeoffset not null,
    updated_at datetimeoffset not null
);

create unique index uk_llm_profile_name on llm_profile (name);
