create table helper_access (
    id bigint generated always as identity primary key,
    secret_hash bytea not null unique,
    label text not null,
    expires_at timestamptz not null default (now() + interval '24 hours')
);

create table helper_tasks (
    access_id bigint not null references helper_access(id) on delete cascade,
    task_id bigint not null references tasks(id) on delete cascade,
    title text not null,
    primary key (access_id, task_id)
);

create index helper_tasks_task_id_idx on helper_tasks(task_id);
