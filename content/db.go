package content

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Comment struct {
	Username string `db:"username"`
	When     string `db:"when"`
	Content  string `db:"content"`
}

type Post struct {
	ID       int       `db:"id"`
	Link     string    `db:"link"`
	Title    string    `db:"title"`
	Summary  string    `db:"summary"`
	Author   string    `db:"author"`
	Content  any       `db:"-"`
	Date     string    `db:"date"`
	Comments []Comment `db:"-"`
	Profile  string    `db:"-"`
}

type Thumbnail struct {
	Link    string `db:"link"`
	Title   string `db:"title"`
	Summary string `db:"summary"`
	Date    string `db:"date"`
}

func New() (*pgxpool.Pool, error) {
	return pgxpool.New(context.Background(), "postgres://postgres@localhost:5432/mysite")
}

func GetThumbnails(pool *pgxpool.Pool, limit int) ([]Thumbnail, error) {
	var rows pgx.Rows
	var err error
	if limit > 0 {
		query := `SELECT link, title, summary, time_format(updated_at) AS date FROM posts ORDER BY updated_at DESC LIMIT $1`
		rows, err = pool.Query(context.Background(), query, limit)
	} else {
		query := `SELECT link, title, summary, time_format(updated_at) AS date FROM posts ORDER BY created_at DESC`
		rows, err = pool.Query(context.Background(), query)
	}
	if err != nil {
		return []Thumbnail{}, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[Thumbnail])
}

func GetPostContent(pool *pgxpool.Pool, link string) (Post, error) {
	query := `
SELECT p.id, link, title, summary, u.username AS author, time_format(updated_at) as date
FROM posts p
JOIN users u ON p.author_id = u.id
WHERE p.link = $1`
	rows, err := pool.Query(context.Background(), query, link)
	defer rows.Close()
	if err != nil {
		return Post{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[Post])
}

func GetComments(pool *pgxpool.Pool, postID int) ([]Comment, error) {
	// SELECT u.username, time_format(c.created_at) AS when, c.content
	query := `
SELECT u.username, time_format(c.created_at) AS when, c.content
FROM comments c
JOIN users u ON c.user_id = u.id
WHERE c.post_id = $1
ORDER BY c.created_at ASC`
	rows, err := pool.Query(context.Background(), query, postID)
	if err != nil {
		return []Comment{}, err
	}
	defer rows.Close()
	comments, err := pgx.CollectRows(rows, pgx.RowToStructByName[Comment])
	if err != nil {
		return []Comment{}, nil
	}
	return comments, err
}

func PostComment(pool *pgxpool.Pool, postID int, userID string, content string) ([]Comment, error) {
	query := `
WITH rows AS
(INSERT INTO comments (post_id, user_id, content) VALUES
 ($1, (SELECT id FROM users WHERE username = $2), $3) RETURNING *)
SELECT u.username, time_format(c.created_at) AS when, c.content
FROM rows c JOIN users u ON
c.user_id = u.id`
	rows, err := pool.Query(context.Background(), query, postID, userID, content)
	defer rows.Close()
	if err != nil {
		return []Comment{}, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Comment])
}
