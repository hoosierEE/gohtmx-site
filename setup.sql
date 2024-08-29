SET TIME ZONE 'UTC';

CREATE TABLE users (
id SERIAL PRIMARY KEY,
username VARCHAR(50) UNIQUE NOT NULL,
email VARCHAR(254) UNIQUE NOT NULL, -- 254 is not a typo
password_hash VARCHAR(255) NOT NULL,
created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE posts (
id SERIAL PRIMARY KEY,
link VARCHAR(75) UNIQUE NOT NULL,
title VARCHAR(255) UNIQUE NOT NULL,
author_id INTEGER NOT NULL,
summary TEXT NOT NULL, -- metadata for link previews or "abstract/tl;dr" sections
content TEXT NOT NULL,
created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
FOREIGN KEY (author_id) REFERENCES users(id) -- post remains after author_id deleted
);

CREATE TABLE comments (
id SERIAL PRIMARY KEY,
post_id INTEGER NOT NULL, -- comments belong to a post
user_id INTEGER NOT NULL,
content TEXT NOT NULL,
created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE, -- delete user's comments if user deleted
FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE -- delete post's comments if post deleted
);

-- dummy values
INSERT INTO users (username, email, password_hash) VALUES
('john_doe', 'john@example.com', 'hashed_password_1'),
('jane_smith', 'jane@example.com', 'hashed_password_2'),
('bob_johnson', 'bob@example.com', 'hashed_password_3');

INSERT INTO posts (author_id, link, title, summary, content) VALUES
(1, 'first-post', 'first post', 'some content', 'this is some content'),
(1, 'second-post', 'hello world', 'heyo', 'hello everyone! this is my post'),
(2, 'third-post', 'postgresql tips', 'article', 'some tips about postgresql');

INSERT INTO comments (post_id, user_id, created_at, content) VALUES
(1, 2, '2024-08-22 14:30:00', 'Great first post!'),
(1, 3, '2024-08-22 14:31:00', 'Looking forward to more content.'),
(2, 1, '2024-08-22 14:32:00', 'Welcome to the blogging world!'),
(2, 3, '2024-08-22 14:33:00', 'Nice start, Jane!'),
(3, 1, '2024-08-22 14:34:00', 'These tips are really helpful.'),
(3, 2, '2024-08-22 14:35:00', 'Thanks for sharing, Bob!');


-- TODO: this might be handled better by always returning UTC to client,
-- then allowing the client to display in local format. But support isn't
-- implemented (yet): https://docs.timetime.in/blog/js-dates-finally-fixed/
-- It still might be better to use a polyfill in the meantime.
CREATE OR REPLACE FUNCTION time_format(timestamp_value TIMESTAMPTZ) RETURNS text AS $$
DECLARE
diff interval;
ago text;
BEGIN
diff := now() - timestamp_value;

IF diff < '1 minute'::interval THEN
ago := 'just now';
ELSIF diff < '1 hour'::interval THEN
ago := (extract(epoch from diff) / 60)::integer || ' minutes ago';
ELSIF diff < '1 day'::interval THEN
ago := (extract(epoch from diff) / 3600)::integer || ' hours ago';
ELSIF diff < '30 days'::interval THEN
ago := (extract(epoch from diff) / 86400)::integer || ' days ago';
ELSIF diff < '1 year'::interval THEN
ago := (extract(epoch from diff) / 2592000)::integer || ' months ago';
ELSE
ago := TO_CHAR(timestamp_value, 'YYYY-MM-DD');
END IF;

RETURN ago;
END;
$$
LANGUAGE plpgsql;
