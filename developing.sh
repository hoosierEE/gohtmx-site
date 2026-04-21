#!/bin/zsh

PSQL=${PSQL:-psql-17}

# remove database
$PSQL -U postgres -d mysite -f cleanup.sql

# initialize or refresh database: ./initdb.sh
$PSQL -U postgres -d mysite -f setup.sql

# add html to database
go run scripts/indexPosts.go public/posts/

# compile and run (recompile as files change)
air
