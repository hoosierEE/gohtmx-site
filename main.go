package main

import (
	"context"
	"html/template"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func assert(e error) {
	if e != nil {
		log.Panic(e)
	}
}

type Site struct {
	Title       string
	Description string
	Thumbs      []Thumbnail
}

type Thumbnail struct {
	Link    string `db:"link"`
	Title   string `db:"title"`
	Summary string `db:"summary"`
}

type Templates struct {
	templates *template.Template
}

func (t *Templates) Render(w io.Writer, name string, data interface{}) error {
	return t.templates.ExecuteTemplate(w, name, data) // named templates woo
}

func getThumbnails(pool *pgxpool.Pool) []Thumbnail {
	query := `SELECT link, title, summary FROM posts`
	rows, err := pool.Query(context.Background(), query)
	assert(err)
	defer rows.Close()
	thums, err := pgx.CollectRows(rows, pgx.RowToStructByName[Thumbnail])
	assert(err)
	return thums
}

type Post struct {
	ID        int       `db:"id"`
	Link      string    `db:"link"`
	Title     string    `db:"title"`
	Summary   string    `db:"summary"`
	Author    string    `db:"author"`
	Content   string    `db:"content"`
	UpdatedAt time.Time `db:"updated_at"`
	Comments  []Comment `db:"-"`
}

func getPostContent(pool *pgxpool.Pool, link string) Post {
	log.Print("link: ", link)
	query := `
SELECT p.id, link, title, summary, u.username AS author, content, updated_at
FROM posts p
JOIN users u ON p.author_id = u.id
WHERE p.link = $1`
	rows, err := pool.Query(context.Background(), query, link)
	assert(err)
	defer rows.Close()
	post, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Post])
	assert(err)
	return post
}

type Comment struct {
	Username string    `db:"username"`
	When     time.Time `db:"when"`
	Content  string    `db:"content"`
}

func getComments(pool *pgxpool.Pool, postID int) []Comment {
	// SELECT u.username, time_format(c.created_at) AS when, c.content
	query := `
SELECT u.username, c.created_at AS when, c.content
FROM comments c
JOIN users u ON c.user_id = u.id
WHERE c.post_id = $1
ORDER BY c.created_at DESC`
	rows, err := pool.Query(context.Background(), query, postID)
	assert(err)
	defer rows.Close()
	comments, err := pgx.CollectRows(rows, pgx.RowToStructByName[Comment])
	assert(err)
	return comments
}

// func addComment(pool *pgxpool.Pool, postID int, username string, content string) int {
// 	// first look up userID from username
// 	uquery := `
// SELECT id FROM users WHERE username=$1`
// 	var userID int
// 	var err error
// 	err = pool.QueryRow(context.Background(), uquery, username).Scan(&userID)
// 	log.Print("ran query, got userID: ", userID, err)
// 	assert(err)
// 	query := `
// INSERT INTO comments (post_id, user_id, content, created_at)
// VALUES ($1, $2, $3, CURRENT_TIMESTAMP)`
// 	var commentID int
// 	err = pool.QueryRow(context.Background(), query, postID, userID, content).Scan(&commentID)
// 	log.Print("ran query, got commentID: ", commentID, err)
// 	assert(err)
// 	return commentID
// }

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres@localhost:5432/mysite")
	assert(err)
	defer pool.Close()

	ts := &Templates{ // parse all templates up front
		template.Must(template.ParseGlob("views/*.html")),
	}

	fileServer := http.FileServer(http.Dir("./static"))         // stored in /static on local fs
	http.Handle("GET /s/", http.StripPrefix("/s/", fileServer)) // called /s in html templates

	http.HandleFunc("GET /posts/{post}", func(w http.ResponseWriter, r *http.Request) {
		post := getPostContent(pool, r.PathValue("post"))
		log.Print(post)

		comments := getComments(pool, post.ID)
		post.Comments = comments
		log.Print(comments)

		err := ts.Render(w, "post", post)
		if err != nil {
			log.Print(err)
		}
	})

	http.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		site := Site{
			Description: "cool website",
			Title:       "muh blog",
			Thumbs:      getThumbnails(pool),
		}
		assert(ts.Render(w, "index", site))
	})

	// http.HandleFunc("/a/result", func(w http.ResponseWriter, r *http.Request) {
	// 	when := time.Now().UTC()
	// 	log.Print("comment arrived at time: ", when.String())
	// 	username := r.PostFormValue("user")
	// 	content := r.PostFormValue("comment")
	// 	commentID := addComment(pool, 1, username, content)
	// 	log.Print("commentID: ", commentID)
	// 	// TODO: put new comment in db
	// 	// then re-render just what changed
	// 	ts.Render(w, "comment", Comment{username, when.String(), content})
	// })

	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
