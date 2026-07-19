create table if not exists participants (
	id bigserial primary key,
	name text not null unique,
	role text not null check (role in ('parent', 'child')),
	created_at timestamptz not null default now()
);

create table if not exists chores (
	id bigserial primary key,
	title text not null unique,
	description text not null default '',
	schedule text not null check (schedule in ('once', 'daily', 'daily_windowed', 'weekly', 'monthly')),
	time_window text not null default '' check (time_window in ('', 'morning', 'day', 'evening')),
	base_value integer not null check (base_value > 0),
	active boolean not null default true,
	created_at timestamptz not null default now()
);

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
