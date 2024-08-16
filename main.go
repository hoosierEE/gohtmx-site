package main

import (
	"html/template"
	"io"
	"log"
	"net/http"
)

type Templates struct {
	templates *template.Template
}

type UserComment struct {
	User    string
	Comment string
}

// this is why we name our templates
func (t *Templates) Render(w io.Writer, name string, data interface{}) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func main() {
	// local "./static" becomes "/s" in html
	fileServer := http.FileServer(http.Dir("./static"))
	http.Handle("/s/", http.StripPrefix("/s/", fileServer))

	// pre-populate some hard-coded data (TODO: from db)
	comments := []UserComment{
		{User: "alex", Comment: "hi"},
		{User: "prateek", Comment: "what's up?"},
	}

	// parse all templates up front
	ts := &Templates{
		templates: template.Must(template.ParseGlob("views/*.html")),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ts.Render(w, "index", struct{ Comments []UserComment }{comments})
	})

	http.HandleFunc("/a/result", func(w http.ResponseWriter, r *http.Request) {
		data := UserComment{
			User:    r.PostFormValue("user"),
			Comment: r.PostFormValue("comment"),
		}
		comments = append(comments, data) // append returns a new array, neato
		ts.Render(w, "comment", data)
	})

	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
