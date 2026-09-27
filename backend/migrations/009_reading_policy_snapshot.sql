-- Short-lived reading starts bind rewards to the policy at lesson start.
-- Временный старт чтения связывает награду с уровнем на начало занятия.
create table if not exists reading_starts (
 id text primary key,
 participant_id bigint not null references participants(id) on delete cascade,
 level text not null check(level in ('phrases','sentences','advanced')),
 policy_version integer not null,
 created_at timestamptz not null
);
