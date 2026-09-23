create table math_sessions (
 id text primary key,
 participant_id bigint not null references participants(id),
 data jsonb not null check (jsonb_typeof(data) = 'object'),
 created_at timestamptz not null default now()
);
create index math_sessions_participant on math_sessions(participant_id,created_at desc);
create table activity_rewards (
 source text not null check(source in ('math','sport','habit','adventure')),
 source_key text not null,
 participant_id bigint not null references participants(id),
 earned_date date not null,
 stars integer not null check(stars>=0 and stars<=100),
 smiles integer not null check(smiles>=0 and smiles<=100),
 title text not null,
 primary key(source,source_key,participant_id)
);
create index activity_rewards_period on activity_rewards(participant_id,earned_date);
