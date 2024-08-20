CREATE TABLE users (
id SERIAL PRIMARY KEY,
username VARCHAR(50) UNIQUE NOT NULL,
email VARCHAR(100) UNIQUE NOT NULL,
password_hash VARCHAR(255) NOT NULL,
created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE posts (
id SERIAL PRIMARY KEY,
user_id INTEGER NOT NULL,
title VARCHAR(100) UNIQUE NOT NULL,
content TEXT NOT NULL,
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE -- delete post if user deleted
);

CREATE TABLE comments (
id SERIAL PRIMARY KEY,
post_id INTEGER NOT NULL,
user_id INTEGER NOT NULL,
content TEXT NOT NULL,
created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE, -- delete user's comments if user deleted
FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE -- delete post's comments if post deleted
);

-- dummy values
INSERT INTO users (username, email, password_hash) VALUES
('john_doe', 'john@example.com', 'hashed_password_1'),
('jane_smith', 'jane@example.com', 'hashed_password_2'),
('bob_johnson', 'bob@example.com', 'hashed_password_3');

INSERT INTO posts (user_id, title, content) VALUES
(1, 'first post', 'this is some content'),
(1, 'hello world', 'hello everyone! this is my post'),
(2, 'postgresql tips', 'some tips about postgresql');

INSERT INTO comments (post_id, user_id, content) VALUES
(1, 2, 'Great first post!'),
(1, 3, 'Looking forward to more content.'),
(2, 1, 'Welcome to the blogging world!');

-- (2, 3, 'Nice start, Jane!'),
-- (3, 1, 'These tips are really helpful.'),
-- (3, 2, 'Thanks for sharing, Bob!');
