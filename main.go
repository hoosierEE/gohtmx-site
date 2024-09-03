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

type User struct {
	Name    string
	Session string
}

type Site struct {
	Title    string
	Summary  string
	Content  string
	Link     string
	Session  string
	Display  string
	Thumbs   []Thumbnail
	Comments []Comment
}

type Thumbnail struct {
	Link    string    `db:"link"`
	Title   string    `db:"title"`
	Summary string    `db:"summary"`
	Date    time.Time `db:"date"`
}

func getThumbnails(pool *pgxpool.Pool) ([]Thumbnail, error) {
	query := `SELECT link, title, summary, updated_at AS date FROM posts`
	rows, err := pool.Query(context.Background(), query)
	defer rows.Close()
	if err != nil {
		return []Thumbnail{}, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Thumbnail])
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
	Display   string    `db:"-"`
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
	assert(err)
	defer rows.Close()
	comments, err := pgx.CollectRows(rows, pgx.RowToStructByName[Comment])
	assert(err)
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

func validLogin(username, password string) bool {
	users := map[string]string{
		"asdf":       "asdf",
		"jane_smith": "asdf",
		"john_doe":   "asdf",
	}
	if val, ok := users[username]; ok {
		if password == val {
			return true
		}
	}
	return false
}

type Templates map[string]*template.Template

func parseTemplates(prefix string) Templates {
	var err error
	t := make(map[string]*template.Template)
	base := template.Must(template.ParseFiles(prefix + "base.html"))
	t["base"] = base
	t["rss"], err = template.Must(base.Clone()).ParseFiles(prefix + "rss.xml")
	if err != nil {
		log.Fatal("error parsing ", prefix+"rss.xml")
	}
	html := []string{"index", "post", "404"}
	for _, h := range html {
		name := prefix + h + ".html"
		t[h], err = template.Must(base.Clone()).ParseFiles(name)
		if err != nil {
			log.Fatal("error parsing ", name, " Error: ", err)
		}
	}
	return t
}

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres@localhost:5432/mysite")
	assert(err)
	defer pool.Close()

	ts := parseTemplates("views/")
	var user User

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

	http.HandleFunc("GET /logout", func(w http.ResponseWriter, r *http.Request) {
		user.Session = ""
		w.Write([]byte(`
<a id="sessionNav" href="/login" hx-get="/login" hx-target="#login-container" hx-swap="outerHTML">Login</a>`))
	})

	http.HandleFunc("GET /login-cancel", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<div id="login-container" style="display:none;"></div>`))
	})

	http.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		assert(ts["base"].ExecuteTemplate(w, "login-container", nil))
	})

	http.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		// login either succeeds or fails
		// if success: - close modal and update .Session
		// else: display fail message then back to login form
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		username := r.PostFormValue("username")
		password := r.PostFormValue("password")
		if validLogin(username, password) {
			sessionToken := "some-random-thing-for:" + username
			http.SetCookie(w, &http.Cookie{
				Name:     "session_token",
				Value:    sessionToken,
				HttpOnly: true,
				Secure:   true,
				MaxAge:   3600,
			})
			user.Name = username
			user.Session = "1234"
			log.Printf("login success [user: %s]", username)
			// w.Write([]byte(`<div id="login-container" hx-swap="outerHTML" style="display:none;"></div>`))
			ts["base"].ExecuteTemplate(w, "login-container", struct{ Display string }{"none"})
			ts["base"].ExecuteTemplate(w, "login-logout", user)
		} else {
			log.Print("[login failure], user: ", username)
			ts["base"].ExecuteTemplate(w, "login-container", struct{ Display string }{"none"})
			// w.Write([]byte(`<p>invalid login</p>`))
			// TODO:
			// w.Write([]byte(`<div id="login-container" style="display:none;"></div>`))
		}
	})

	http.HandleFunc("GET /posts/{post}", func(w http.ResponseWriter, r *http.Request) {
		data, err := getPostContent(pool, r.PathValue("post"))
		if err != nil {
			assert(ts["404"].ExecuteTemplate(w, "404", nil))
			log.Print(err)
			return
		}
		merged := struct {
			Post
			User
		}{data, user}
		comments := getComments(pool, data.ID)
		merged.Comments = comments
		merged.Display = "none"
		if val, ok := ts["post"]; ok {
			err := val.ExecuteTemplate(w, "post", merged)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			log.Print("no key `post` in ts")
		}
	})

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		site := Site{
			Title:   "Alex Shroyer",
			Summary: "research and hobbies of a computer engineer",
			Display: "none",
			Session: user.Session, // TODO implement actual session handling in db
		}

		site.Thumbs, err = getThumbnails(pool)
		if err != nil {
			assert(ts["404"].ExecuteTemplate(w, "404", nil))
			return
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
