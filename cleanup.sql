-- Drop tables in reverse order of creation to avoid foreign key constraint issues
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS posts;
DROP TABLE IF EXISTS users;
