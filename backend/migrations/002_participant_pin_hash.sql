-- PIN hashing is performed by the application with bcrypt. pin_code remains
-- temporarily nullable so existing databases can migrate credentials lazily
-- after a successful login. New credentials must only populate pin_hash.
alter table participants
	add column if not exists pin_hash text;

alter table participants
	alter column pin_code drop not null,
	alter column pin_code drop default;

