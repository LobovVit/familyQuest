-- Defaults preserve every existing family policy.
alter table service_policy add column if not exists children_unlimited boolean not null default false;
alter table service_policy add column if not exists billing_managed boolean not null default false;
