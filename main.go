package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	// local pacakges
	"siteserver/content"
	"siteserver/users"

	// third party
	"github.com/google/uuid"
)

type Site struct {
	Title   string
	Summary string
	Content any
	Profile string
	Thumbs  []content.Thumbnail
}

type session struct {
	username string
	expires  time.Time
}

// TODO: replace globals
var sessions = map[string]session{}

func (s *session) isExpired() bool {
	return s.expires.Before(time.Now())
}

func validSession(token string) (session, bool) {
	data, ok := sessions[token] // TODO: replace globals
	if ok && !data.isExpired() {
		return data, ok
	}
	return session{}, false
}

func getSession(r *http.Request) (session, bool) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		token := cookie.Value
		return validSession(token)
	}
	return session{}, false
}

func main() {
	pool, err := content.New()
	if err != nil {
		log.Panic(err)
		return
	}
	defer pool.Close()

	var ts Templates = parseTemplates("views/")

	fileServer := http.FileServer(http.Dir("./static")) // "/static" (on local fs)
	imageServer := http.FileServer(http.Dir("./static/images"))
	http.Handle("GET /s/", http.StripPrefix("/s/", fileServer)) // "/s" (in html templates)
	http.Handle("GET /images/", http.StripPrefix("/images/", imageServer))

	http.HandleFunc("POST /posts/{link}/comment", func(w http.ResponseWriter, r *http.Request) {
		data, err := content.GetPostContent(pool, r.PathValue("link"))
		if err != nil {
			log.Print("POST /posts/{link}/comment content.GetPostContent() failed:", err)
			return
		}
		comment := r.PostFormValue("comment")
		if len(comment) < 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if sess, ok := getSession(r); ok {
			data.Profile = sess.username
		}

		// TODO: update with actual users from users package
		userExists, err := users.Exists(pool, data.Profile)
		if err != nil {
			log.Print("users.Exists: ", err)
			return
		}
		if userExists {
			comments, err := content.PostComment(pool, data.ID, data.Profile, comment)
			if err != nil {
				log.Print(err)
				return
			}
			assert(ts["post"].ExecuteTemplate(w, "oob-comment", comments[0])) // update the comments
			assert(ts["post"].ExecuteTemplate(w, "form", data))               // replace form with an empty one
		}
	})

	http.HandleFunc("GET /profile", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := getSession(r); ok {
			return
		}
		assert(ts["profile"].ExecuteTemplate(w, "profile", nil))
	})

	http.HandleFunc("GET /login-cancel", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<div id="login-container" class="invisible"></div>`))
	})

	http.HandleFunc("GET /logout", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session_token")
		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				goto cleanup
			}
			w.WriteHeader(http.StatusBadRequest)
			goto cleanup
		}
		delete(sessions, c.Value)
	cleanup:
		// TODO: could this be better handled somewhere else?
		parsedURL, err := url.Parse(r.Referer())
		if err != nil {
			log.Print("error parsing URL from r.Referer(): ", r.Referer())
			log.Print(err)
		} else {
			pathParts := strings.Split(parsedURL.Path, "/") //  "/posts" ⇒ ["", "posts"]
			if len(pathParts) > 2 && pathParts[1] == "posts" {
				ts["post"].ExecuteTemplate(w, "form", struct {
					Profile string
					Link    string
				}{"", pathParts[2]})
			}
		}
		log.Print("logged out user")
		http.SetCookie(w, &http.Cookie{Name: "session_token", Value: "", Expires: time.Now()})
		w.Write([]byte(`<a id="login-logout" href="#" hx-get="/profile" hx-target="#login-target">Login</a>`))
	})

	http.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Print("r.ParseForm():", err)
			return
		}
		username := r.FormValue("username")
		password := r.FormValue("password")
		user, err := users.Get(pool, username)
		if err != nil {
			log.Print("users.Get(\"", username, "\") error:", err)
		}
		match, err := users.ComparePW(password, user.Pass)
		if err != nil {
			log.Print("users.ComparePW() fail:", err)
		}
		if match {
			sessionToken := uuid.NewString()
			expiresAt := time.Now().Add(3600 * time.Second) // auto logout after 60*60 seconds
			sessions[sessionToken] = session{
				username: username,
				expires:  expiresAt,
			}
			http.SetCookie(w, &http.Cookie{
				Name:    "session_token",
				Value:   sessionToken,
				Expires: expiresAt,
			})
			w.Write([]byte(`<div id="login-container" class="invisible"></div>`))
			w.Write([]byte(`<a id="login-logout" hx-swap-oob="true" hx-swap="outerHTML" href="#" hx-get="/logout">Logout ` + username + `</a>`))

			// TODO: could this be better handled somewhere else?
			// if we're on a post page, there's an add-comment box that should appear after login succeeds
			parsedURL, err := url.Parse(r.Referer())
			if err != nil {
				log.Print("url.Parse(r.Referer()): ", err)
				return
			}
			pathParts := strings.Split(parsedURL.Path, "/")
			if len(pathParts) > 2 && pathParts[1] == "posts" {
				ts["post"].ExecuteTemplate(w, "form", struct {
					Profile string
					Link    string
				}{username, pathParts[2]})
			}
		} else {
			log.Print("unauthorized user login attempt:", username)
			w.WriteHeader(http.StatusUnauthorized)
			data := struct {
				Username string
				Error    string
			}{username, "Incorrect username or password"}
			assert(ts["profile"].ExecuteTemplate(w, "profile", data))
		}
	})

	http.HandleFunc("GET /posts/{link}", func(w http.ResponseWriter, r *http.Request) {
		link := r.PathValue("link")
		data, err := content.GetPostContent(pool, link)
		if err != nil {
			assert(ts["404"].ExecuteTemplate(w, "404", nil))
			log.Printf("GET /posts/{link} content.GetPostContent failed: %v", err)
			return
		}
		// TODO: better handled elsewhere?
		if sess, ok := getSession(r); ok {
			data.Profile = sess.username
		}
		file, err := os.Open("./public/posts/" + link + ".html")
		if err != nil {
			log.Printf("GET /posts/{link} err:%v", err)
			return
		}
		defer file.Close()
		fileContent, err := ioutil.ReadAll(file)
		if err != nil {
			log.Printf("ioutil.Readall(file) error: %v", err)
			return
		}
		data.Content = template.HTML(string(fileContent)) // what type?
		data.Comments, err = content.GetComments(pool, data.ID)
		if err != nil {
			log.Print("content.GetComments: ", err)
		}
		if val, ok := ts["post"]; ok {
			err := val.ExecuteTemplate(w, "post", data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			log.Print("no key `post` in ts")
		}
	})

	http.HandleFunc("GET /cv", func(w http.ResponseWriter, r *http.Request) {
		file, err := os.Open("./public/pages/cv.html")
		if err != nil {
			log.Printf("GET /cv err:%v", err)
			return
		}
		defer file.Close()
		fileContent, err := ioutil.ReadAll(file)
		if err != nil {
			log.Printf("ioutil.Readall(file) error: %v", err)
			return
		}
		data := Site{}
		// TODO: better handled elsewhere?
		if sess, ok := getSession(r); ok {
			data.Profile = sess.username
		}
		data.Content = template.HTML(string(fileContent)) // what type?
		if val, ok := ts["cv"]; ok {
			err := val.ExecuteTemplate(w, "cv", data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			log.Print("no key `cv` in ts")
		}
	})

	http.HandleFunc("GET /posts", func(w http.ResponseWriter, r *http.Request) {
		site := Site{}
		if sess, ok := getSession(r); ok {
			site.Profile = sess.username
		}
		site.Thumbs, err = content.GetThumbnails(pool, -1)
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
			Thumbs:  []content.Thumbnail{},
		}
		if sess, ok := getSession(r); ok {
			site.Profile = sess.username
		}
		switch r.URL.String() {
		case "/":
			if site.Thumbs, err = content.GetThumbnails(pool, 4); err != nil {
				log.Printf("[thumbnails] %v", err)
				assert(ts["404"].ExecuteTemplate(w, "404", nil))
			}
			assert(ts["index"].ExecuteTemplate(w, "index", site))
		case "/rss.xml":
			if site.Thumbs, err = content.GetThumbnails(pool, -1); err != nil {
				log.Printf("[thumbnails] %v", err)
				assert(ts["404"].ExecuteTemplate(w, "404", nil))
			}
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

func assert(e ...any) {
	if e[0] != nil {
		log.Fatal(e...)
	}
}

type Templates map[string]*template.Template

func parseTemplates(prefix string) Templates {
	var err error
	t := Templates{} //make(map[string]*template.Template)
	base := template.Must(template.ParseFiles(prefix + "base.html"))
	t["base"] = base
	t["rss"], err = template.Must(base.Clone()).ParseFiles(prefix + "rss.xml")
	assert(err, "error parsing ", prefix+"rss.xml")
	t["profile"] = template.Must(template.ParseFiles(prefix + "profile.html"))
	html := []string{
		"404",
		"cv",
		"index",
		"post",
		"posts",
	}
	for _, h := range html {
		name := prefix + h + ".html"
		t[h], err = template.Must(base.Clone()).ParseFiles(name)
		assert(err, "error parsing ", name)
	}
	return t
}
