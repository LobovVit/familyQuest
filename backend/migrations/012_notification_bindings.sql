-- Consent and revocation now live in Core PostgreSQL, independently of Access files.
-- Legacy unbound consent cannot be upgraded without a fresh, verified Portal binding.
alter table notification_recipients add column binding_id text not null default '';
alter table notification_outbox add column binding_id text not null default '';
update notification_recipients set enabled=false,updated_at=now();
update notification_outbox set state='cancelled',error_code='binding_upgrade',
    lease_token=null,lease_until=null,settled_at=now() where state='pending';
alter table notification_recipients add constraint notification_binding_required
    check(not enabled or (left(binding_id,4)='tgb_' and length(binding_id) between 5 and 128));
create index notification_outbox_pending_recipient
    on notification_outbox(recipient_ref,subscription_id,binding_id) where state='pending';
