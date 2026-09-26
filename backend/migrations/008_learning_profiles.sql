-- Child settings belong to a family schema. / Настройки ребёнка принадлежат схеме семьи.
alter table participants add column if not exists birth_date date;
alter table participants add column if not exists math_level text not null default '' check(math_level in ('','easy','medium','hard','columnar'));
alter table participants add column if not exists reading_level text not null default '' check(reading_level in ('','phrases','sentences','advanced'));
create table if not exists service_policy (
 id boolean primary key default true check(id),
 child_limit integer not null check(child_limit >= 0),
 access_until timestamptz,
 owner_participant_id bigint
);
insert into service_policy(id,child_limit) values(true,1000) on conflict do nothing;
