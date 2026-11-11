drop index uk_db_connection_name on db_connection;

create unique index uk_db_connection_owner_name on db_connection (created_by, name);
