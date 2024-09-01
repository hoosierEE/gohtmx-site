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

type Site struct {
	Title    string
	Summary  string
	Content  string
	Link     string
	Session  string
	Thumbs   []Thumbnail
	Comments []Comment
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
	Session   string    `db:"-"`
	UpdatedAt time.Time `db:"updated_at"`
	Comments  []Comment `db:"-"`
}

func getPostContent(pool *pgxpool.Pool, link string) Post {
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
	Username string `db:"username"`
	When     string `db:"when"` // When     time.Time `db:"when"`
	Content  string `db:"content"`
}

func getComments(pool *pgxpool.Pool, postID int) []Comment {
	// SELECT u.username, time_format(c.created_at) AS when, c.content
	query := `
SELECT u.username, time_format(c.created_at) AS when, c.content
FROM comments c
JOIN users u ON c.user_id = u.id
WHERE c.post_id = $1
ORDER BY c.created_at ASC`
	rows, err := pool.Query(context.Background(), query, postID)
	assert(err)
	defer rows.Close()
	comments, err := pgx.CollectRows(rows, pgx.RowToStructByName[Comment])
	assert(err)
	return comments
}

func postComment(pool *pgxpool.Pool, postID, userID int, content string) []Comment {
	query := `
WITH rows AS
(INSERT INTO comments (post_id, user_id, content) VALUES ($1, $2, $3) RETURNING *)
SELECT u.username, time_format(c.created_at) AS when, c.content
FROM rows c JOIN users u ON
c.user_id = u.id`
	rows, err := pool.Query(context.Background(), query, postID, userID, content)
	defer rows.Close()
	assert(err)
	comment, err := pgx.CollectRows(rows, pgx.RowToStructByName[Comment])
	assert(err)
	return comment
}

func parseTemplates(prefix string) map[string]*template.Template {
	var err error
	base := template.Must(template.ParseFiles(prefix + "base.html"))
	html := [3]string{"index", "post", "404"}
	t := make(map[string]*template.Template)
	t["rss"], err = template.Must(base.Clone()).ParseFiles(prefix + "rss.xml")
	if err != nil {
		log.Fatal("error parsing ", prefix+"rss.xml")
	}
	for _, h := range html {
		name := prefix + h + ".html"
		t[h], err = template.Must(base.Clone()).ParseFiles(name)
		if err != nil {
			log.Fatal("error parsing ", name)
		}
	}
	return t
}

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres@localhost:5432/mysite")
	assert(err)
	defer pool.Close()

	ts := parseTemplates("views/")

	fileServer := http.FileServer(http.Dir("./static"))         // "/static" (on local fs)
	http.Handle("GET /s/", http.StripPrefix("/s/", fileServer)) // "/s" (in html templates)

	http.HandleFunc("POST /posts/{link}/comment", func(w http.ResponseWriter, r *http.Request) {
		data := getPostContent(pool, r.PathValue("link"))
		comment := r.PostFormValue("comment")
		comments := postComment(pool, data.ID, 1, comment)
		err = ts["post"].ExecuteTemplate(w, "oob-comment", comments[0])
		err = ts["post"].ExecuteTemplate(w, "form", data) // replace the form with an empty form
		assert(err)
	})

	http.HandleFunc("GET /posts/{post}", func(w http.ResponseWriter, r *http.Request) {
		data := getPostContent(pool, r.PathValue("post"))
		comments := getComments(pool, data.ID)
		data.Comments = comments
		err := ts["post"].ExecuteTemplate(w, "post", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		site := Site{
			Title:   "Alex Shroyer",
			Summary: "research and hobbies of a computer engineer",
		}
		switch r.URL.String() {
		case "/":
			site.Thumbs = getThumbnails(pool)
			assert(ts["index"].ExecuteTemplate(w, "index", site))
		case "/rss.xml":
			site.Thumbs = getThumbnails(pool)
			w.Header().Set("Content-Type", "application/xml")
			assert(ts["rss"].ExecuteTemplate(w, "rss", site))
		default:
			w.WriteHeader(http.StatusNotFound)
			assert(ts["404"].ExecuteTemplate(w, "404", site))
		}
	})

	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
