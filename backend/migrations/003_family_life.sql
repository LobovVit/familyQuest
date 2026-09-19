-- One installation represents one family, as do the existing participants/tasks.
create table family_entries (
 id bigint generated always as identity primary key,
 version integer not null default 1 check (version > 0),
 author_id bigint not null references participants(id),
 created_at timestamptz not null default now(),
 updated_at timestamptz not null default now(),
 data jsonb not null check (jsonb_typeof(data) = 'object')
);
