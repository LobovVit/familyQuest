-- Add reading to the existing reward ledger. / Чтение в общем журнале наград.
alter table activity_rewards drop constraint activity_rewards_source_check;
alter table activity_rewards add constraint activity_rewards_source_check
 check (source in ('math','sport','habit','adventure','reading'));
