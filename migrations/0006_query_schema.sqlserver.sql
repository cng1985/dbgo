alter table query_history add schema_name nvarchar(128) not null constraint df_query_history_schema default N'';
