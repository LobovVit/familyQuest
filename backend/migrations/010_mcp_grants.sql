create table mcp_grants (
 id text primary key,
 participant_id bigint not null references participants(id) on delete cascade,
 session_version bigint not null,
 name text not null check (length(name) between 1 and 80),
 read_only boolean not null,
 created_at timestamptz not null,
 expires_at timestamptz not null,
 revoked_at timestamptz
);
create index mcp_grants_owner on mcp_grants(participant_id);
