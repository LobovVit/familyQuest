-- Device secrets are random; only their SHA-256 digest is persisted.
create table trusted_devices (
 id text primary key,
 participant_id bigint not null references participants(id) on delete cascade,
 session_version bigint not null,
 token_hash text not null unique,
 name text not null check (length(name) between 1 and 80),
 created_at timestamptz not null default now(),
 last_seen_at timestamptz not null default now(),
 expires_at timestamptz not null,
 revoked_at timestamptz
);
create index trusted_devices_owner on trusted_devices(participant_id);
