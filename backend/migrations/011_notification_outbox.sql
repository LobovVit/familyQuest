-- Private integration metadata: no child names, PINs, initData or Portal session tokens.
-- No FKs to business rows: backup restore explicitly cancels pending work, never truncates history.
create table notification_recipients (
    recipient_ref text primary key check(length(recipient_ref) between 1 and 128),
    subscription_id uuid not null,
    issuer text not null,
    subject text not null,
    parent_id bigint not null check(parent_id > 0),
    enabled boolean not null default true,
    updated_at timestamptz not null default now()
);
create table notification_outbox (
    id uuid primary key default gen_random_uuid(),
    task_id bigint not null,
    recipient_ref text not null,
    subscription_id uuid not null,
    issuer text not null,
    subject text not null,
    parent_id bigint not null,
    state text not null default 'pending' check(state in ('pending','delivered','cancelled','failed')),
    attempts integer not null default 0 check(attempts >= 0),
    available_at timestamptz not null default now(),
    lease_token uuid,
    lease_until timestamptz,
    error_code text not null default '',
    portal_id text not null default '',
    created_at timestamptz not null default now(),
    settled_at timestamptz,
    check((lease_token is null) = (lease_until is null))
);
create index notification_outbox_pending on notification_outbox(available_at,created_at) where state='pending';
