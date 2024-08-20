package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type UserComment struct {
	User    string
	Comment string
}

type Templates struct {
	templates *template.Template
}

// this is why we name our templates
func (t *Templates) Render(w io.Writer, name string, data interface{}) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func main() {
	// local "./static" becomes "/s" in html
	fileServer := http.FileServer(http.Dir("./static"))
	http.Handle("/s/", http.StripPrefix("/s/", fileServer))

	// pg
	urlExample := "postgres://ashroyer-admin@localhost:5432/ashroyer-admin"
	conn, err := pgx.Connect(context.Background(), urlExample)
	if err != nil {
		log.Fatal("Unable to connect to db:", err)
	}
	defer conn.Close(context.Background())

	var greeting string
	err = conn.QueryRow(context.Background(), "select 'Hello, world!'").Scan(&greeting)
	if err != nil {
		log.Fatal("QueryRow failed: ", err)
	}
	fmt.Println(greeting)

	// pre-populate some hard-coded data (TODO: from db)
	comments := []UserComment{
		{User: "alex", Comment: "hi"},
		{User: "prateek", Comment: "what's up?"},
	}

	ts := &Templates{ // parse all templates up front
		template.Must(template.ParseGlob("views/*.html")),
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
