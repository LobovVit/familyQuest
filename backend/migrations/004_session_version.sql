-- Revoke existing sessions after a PIN change or a database restore.
alter table participants add column session_version bigint not null default 0;
