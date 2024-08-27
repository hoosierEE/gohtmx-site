#!/usr/bin/env sh

# initialize or refresh database: ./initdb.sh

psql -U postgres -d mysite -f cleanup.sql
psql -U postgres -d mysite -f setup.sql
