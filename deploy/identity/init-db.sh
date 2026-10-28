#!/bin/sh
set -eu
# Generated password is hexadecimal; never accept arbitrary SQL fragments.
# Сгенерированный пароль состоит из hex; произвольные SQL-фрагменты запрещены.
case "$IDENTITY_DB_PASSWORD" in *[!0-9a-f]*|'') exit 1;; esac
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<SQL
CREATE ROLE zitadel LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '$IDENTITY_DB_PASSWORD';
ALTER DATABASE zitadel OWNER TO zitadel;
SQL
