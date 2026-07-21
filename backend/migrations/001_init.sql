create table if not exists participants (
	id bigserial primary key,
	name text not null unique,
	role text not null check (role in ('parent', 'child')),
	pin_code text not null default '000000' check (pin_code ~ '^[0-9]{6}$'),
	active boolean not null default true,
	created_at timestamptz not null default now()
);

alter table participants add column if not exists pin_code text not null default '000000';
alter table participants add column if not exists active boolean not null default true;

create table if not exists chores (
	id bigserial primary key,
	title text not null unique,
	description text not null default '',
	schedule text not null check (schedule in ('once', 'daily', 'weekly', 'monthly')),
	time_window text not null default '' check (time_window in ('', 'morning', 'day', 'evening')),
	benefit_type text not null default 'self' check (benefit_type in ('self', 'family', 'care', 'home')),
	execution_mode text not null default 'assigned' check (execution_mode in ('assigned', 'together', 'adult_child', 'anyone')),
	base_value integer not null check (base_value > 0),
	active boolean not null default true,
	created_at timestamptz not null default now()
);

alter table chores drop constraint if exists chores_schedule_check;
update chores set schedule = 'daily' where schedule = 'daily_windowed';
alter table chores add constraint chores_schedule_check check (schedule in ('once', 'daily', 'weekly', 'monthly'));
alter table chores add column if not exists benefit_type text not null default 'self';
alter table chores add column if not exists execution_mode text not null default 'assigned';
alter table chores drop constraint if exists chores_benefit_type_check;
alter table chores add constraint chores_benefit_type_check check (benefit_type in ('self', 'family', 'care', 'home'));
alter table chores drop constraint if exists chores_execution_mode_check;
alter table chores add constraint chores_execution_mode_check check (execution_mode in ('assigned', 'together', 'adult_child', 'anyone'));

create table if not exists assignments (
	id bigserial primary key,
	chore_id bigint not null references chores(id) on delete cascade,
	participant_id bigint not null references participants(id) on delete cascade,
	active boolean not null default true,
	created_at timestamptz not null default now(),
	unique (chore_id, participant_id)
);

create table if not exists tasks (
	id bigserial primary key,
	assignment_id bigint not null references assignments(id) on delete cascade,
	due_date date not null,
	status text not null default 'pending' check (status in ('pending', 'completed', 'needs_work', 'confirmed')),
	completed_by bigint references participants(id),
	completed_at timestamptz,
	confirmed_at timestamptz,
	created_at timestamptz not null default now(),
	unique (assignment_id, due_date)
);

create table if not exists confirmations (
	id bigserial primary key,
	task_id bigint not null references tasks(id) on delete cascade,
	participant_id bigint not null references participants(id) on delete cascade,
	rating integer not null check (rating between 1 and 5),
	comment text not null default '',
	created_at timestamptz not null default now(),
	unique (task_id, participant_id)
);

create table if not exists behavior_ratings (
	id bigserial primary key,
	rated_date date not null,
	rater_participant_id bigint not null references participants(id) on delete cascade,
	target_participant_id bigint not null references participants(id) on delete cascade,
	rating integer not null check (rating between 1 and 5),
	comment text not null default '',
	created_at timestamptz not null default now(),
	unique (rated_date, rater_participant_id, target_participant_id),
	check (rater_participant_id <> target_participant_id)
);

create table if not exists rewards (
	id bigserial primary key,
	title text not null,
	description text not null default '',
	period text not null check (period in ('day', 'week', 'month')),
	reward_type text not null check (reward_type in ('champion', 'stars', 'smiles')),
	star_cost integer not null default 0 check (star_cost >= 0),
	smile_cost integer not null default 0 check (smile_cost >= 0),
	active boolean not null default true,
	created_at timestamptz not null default now()
);

create unique index if not exists rewards_title_unique on rewards (title);
alter table rewards add column if not exists smile_cost integer not null default 0;
alter table rewards drop constraint if exists rewards_reward_type_check;
alter table rewards add constraint rewards_reward_type_check check (reward_type in ('champion', 'stars', 'smiles'));

create table if not exists reward_participants (
	id bigserial primary key,
	reward_id bigint not null references rewards(id) on delete cascade,
	participant_id bigint not null references participants(id) on delete cascade,
	active boolean not null default true,
	created_at timestamptz not null default now(),
	unique (reward_id, participant_id)
);
