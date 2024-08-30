package main

import (
	"context"
	"html/template"
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

type PageData struct {
}

type Site struct {
	Title   string
	Summary string
	Content string
	Thumbs  []Thumbnail
}

type Thumbnail struct {
	Link    string    `db:"link"`
	Title   string    `db:"title"`
	Summary string    `db:"summary"`
	Date    time.Time `db:"date"`
}

func getThumbnails(pool *pgxpool.Pool) []Thumbnail {
	query := `SELECT link, title, summary, updated_at AS date FROM posts`
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

func herr(w http.ResponseWriter, e error) {
	http.Error(w, e.Error(), http.StatusInternalServerError)
}

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres@localhost:5432/mysite")
	assert(err)
	defer pool.Close()

	// parse templates
	ts := template.Must(template.ParseGlob("views/*"))

	fileServer := http.FileServer(http.Dir("./static"))         // stored in /static on local fs
	http.Handle("GET /s/", http.StripPrefix("/s/", fileServer)) // called /s in html templates

	http.HandleFunc("POST /posts/{post}/comments", func(w http.ResponseWriter, r *http.Request) {
		log.Print("post:", r.PathValue("post"))
		comment := r.PostFormValue("comment")
		log.Print("comment:", comment)
	})

	http.HandleFunc("GET /posts/{post}", func(w http.ResponseWriter, r *http.Request) {
		post := getPostContent(pool, r.PathValue("post"))
		comments := getComments(pool, post.ID)
		post.Comments = comments
		err := ts.ExecuteTemplate(w, "post", post)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		site := Site{
			Title:   "Alex Shroyer",
			Summary: "research and hobbies of a computer engineer",
			Content: "",
			Thumbs:  nil,
		}
		pat := r.URL.String()
		switch pat {
		case "/":
			site.Thumbs = getThumbnails(pool)
			assert(ts.ExecuteTemplate(w, "index", site))
		case "/rss.xml":
			site.Thumbs = getThumbnails(pool)
			w.Header().Set("Content-Type", "application/xml")
			assert(ts.ExecuteTemplate(w, "rss", site))
		default:
			assert(ts.ExecuteTemplate(w, "index", site))
		}
	})

	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
