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

type Site struct {
	Title   string
	Summary string
	Content string
	Profile string
	Thumbs  []Thumbnail
}

var users = map[string]string{
	"asdf":     "asdf",
	"john_doe": "asdf",
}

var sessions = map[string]Session{}

type Session struct {
	username string
	expires  time.Time
}

func (s *Session) isExpired() bool {
	return s.expires.Before(time.Now())
}

func validLogin(username, password string) bool {
	val, ok := users[username]
	return ok && val == password
}

func validSession(token string) (Session, bool) {
	data, ok := sessions[token]
	if ok && !data.isExpired() {
		return data, ok
	}
	return Session{}, false
}

func getSession(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		token := cookie.Value
		return validSession(token)
	}
	return Session{}, false
}

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres@localhost:5432/mysite")
	if err != nil {
		log.Panic(err)
		return
	}
	defer pool.Close()

	var ts Templates = parseTemplates("views/")

	fileServer := http.FileServer(http.Dir("./static"))         // "/static" (on local fs)
	http.Handle("GET /s/", http.StripPrefix("/s/", fileServer)) // "/s" (in html templates)

	http.HandleFunc("POST /posts/{link}/comment", func(w http.ResponseWriter, r *http.Request) {
		data, err := getPostContent(pool, r.PathValue("link"))
		if err != nil {
			log.Print(err)
			return
		}
		comments, err := postComment(pool, data.ID, 1, r.PostFormValue("comment"))
		if err != nil {
			log.Print(err)
			return
		}
		if val, ok := ts["post"]; ok {
			assert(val.ExecuteTemplate(w, "oob-comment", comments[0])) // update the comments
			assert(val.ExecuteTemplate(w, "form", data))               // replace form with an empty one
		}
	})

	http.HandleFunc("GET /profile", func(w http.ResponseWriter, r *http.Request) {
		assert(ts["profile"].ExecuteTemplate(w, "profile", nil))
	})

	http.HandleFunc("GET /login-cancel", func(w http.ResponseWriter, r *http.Request) {})

	http.HandleFunc("GET /logout", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<a id="login-logout" href="#" hx-get="/profile" hx-trigger="click" hx-target="#login-target">Login</a>`))
	})

	http.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Print(err)
		}
		username := r.FormValue("username")
		password := r.FormValue("password")
		if validLogin(username, password) {
			w.Write([]byte(`<div hx-swap-oob="true" id="login-target" hx-target="#login-target"></div>`))
			w.Write([]byte(`<a hx-swap-oob="true" id="login-logout" href="#" hx-get="/logout" hx-target="#login-logout">Logout</a>`))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			data := struct {
				Username string
				Error    string
			}{username, "Incorrect username or password"}
			assert(ts["profile"].ExecuteTemplate(w, "profile", data))
		}
	})

	http.HandleFunc("GET /posts/{post}", func(w http.ResponseWriter, r *http.Request) {
		data, err := getPostContent(pool, r.PathValue("post"))
		if err != nil {
			assert(ts["404"].ExecuteTemplate(w, "404", nil))
			log.Printf("getPostContent failed: %v", err)
			return
		}

		data.Comments = getComments(pool, data.ID)
		if val, ok := ts["post"]; ok {
			err := val.ExecuteTemplate(w, "post", data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			log.Print("no key `post` in ts")
		}
	})

	http.HandleFunc("GET /posts", func(w http.ResponseWriter, r *http.Request) {
		site := Site{
			Title:   "Posts",
			Summary: "all posts",
			Content: "",
			Profile: "",
			Thumbs:  []Thumbnail{},
		}

		site.Thumbs, err = getThumbnails(pool, -1)
		if err != nil {
			log.Printf("[thumbnails] %v", err)
			assert(ts["404"].ExecuteTemplate(w, "404", nil))
		}
		assert(ts["posts"].ExecuteTemplate(w, "posts", site))
	})

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		site := Site{
			Title:   "Alex Shroyer",
			Summary: "research and hobbies of a computer engineer",
			Content: "",
			Profile: "",
			Thumbs:  []Thumbnail{},
		}

		if site.Thumbs, err = getThumbnails(pool, 2); err != nil {
			log.Printf("[thumbnails] %v", err)
			assert(ts["404"].ExecuteTemplate(w, "404", nil))
		}

		switch r.URL.String() {
		case "/":
			assert(ts["index"].ExecuteTemplate(w, "index", site))
		case "/rss.xml":
			w.Header().Set("Content-Type", "application/xml")
			assert(ts["rss"].ExecuteTemplate(w, "rss", site))
		default:
			w.WriteHeader(http.StatusNotFound)
			site.Title = "not found"
			assert(ts["404"].ExecuteTemplate(w, "404", site))
		}
	})

	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}

func getThumbnails(pool *pgxpool.Pool, limit int) ([]Thumbnail, error) {
	var rows pgx.Rows
	var err error
	if limit > 0 {
		query := `SELECT link, title, summary, updated_at AS date FROM posts LIMIT $1`
		rows, err = pool.Query(context.Background(), query, limit)
	} else {
		query := `SELECT link, title, summary, updated_at AS date FROM posts`
		rows, err = pool.Query(context.Background(), query)
	}
	if err != nil {
		return []Thumbnail{}, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[Thumbnail])
}

func assert(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

type Thumbnail struct {
	Link    string    `db:"link"`
	Title   string    `db:"title"`
	Summary string    `db:"summary"`
	Date    time.Time `db:"date"`
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
	Profile   string    `db:"-"`
}

func getPostContent(pool *pgxpool.Pool, link string) (Post, error) {
	query := `
SELECT p.id, link, title, summary, u.username AS author, content, updated_at
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

type Comment struct {
	Username string `db:"username"`
	When     string `db:"when"`
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
	if err != nil {
		log.Printf("err:%v", err)
	}
	defer rows.Close()
	comments, err := pgx.CollectRows(rows, pgx.RowToStructByName[Comment])
	if err != nil {
		log.Printf("err:%v")
		return []Comment{}
	}
	return comments
}

func postComment(pool *pgxpool.Pool, postID, userID int, content string) ([]Comment, error) {
	query := `
WITH rows AS
(INSERT INTO comments (post_id, user_id, content) VALUES ($1, $2, $3) RETURNING *)
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

type Templates map[string]*template.Template

func parseTemplates(prefix string) Templates {
	var err error
	t := Templates{} //make(map[string]*template.Template)
	base := template.Must(template.ParseFiles(prefix + "base.html"))
	t["base"] = base
	t["rss"], err = template.Must(base.Clone()).ParseFiles(prefix + "rss.xml")
	if err != nil {
		log.Fatal("error parsing ", prefix+"rss.xml")
	}
	t["profile"] = template.Must(template.ParseFiles(prefix + "profile.html"))
	html := []string{
		"404",
		"index",
		"post",
		"posts",
	}
	for _, h := range html {
		name := prefix + h + ".html"
		t[h], err = template.Must(base.Clone()).ParseFiles(name)
		if err != nil {
			log.Fatal("error parsing ", name, " Error: ", err)
		}
	}
	return t
}
