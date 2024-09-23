package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/html"
)

// Usage:
// go run indexPosts.go ../public/posts/
func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres@localhost:5432/mysite")
	if err != nil {
		log.Panic(err)
	}
	paths, err := os.ReadDir(os.Args[1])
	if err != nil {
		log.Panic(err)
	}
	for _, path := range paths {
		err := addPost(pool, path, "alex_shroyer")
		if err != nil {
			log.Print(path)
			log.Panic(err)
		}
	}
}

func addPost(pool *pgxpool.Pool, path os.DirEntry, author string) error {
	nom := path.Name()
	title := strings.Join(strings.Split(nom[:len(nom)-5], "-")[3:], " ")
	date := nom[:10]
	query := `
	INSERT INTO posts (link, title, author_id, summary, created_at, updated_at)
	VALUES ($1, $2, (SELECT id FROM users WHERE username = $3), $4, $5, $5)`
	file, err := os.Open("./public/posts/" + nom)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()
	htmlContent, err := ioutil.ReadAll(file)
	if err != nil {
		log.Panic(err)
	}
	doc, err := html.Parse(strings.NewReader(string(htmlContent)))
	if err != nil {
		log.Panic(err)
	}
	summary := findNodeByClass(doc, "abstract")
	if summary == nil {
		return nil
	}
	_, err = pool.Exec(
		context.Background(),
		query,
		nom[:len(nom)-5],
		title,
		author,
		extractText(summary),
		date,
	)
	summ := extractText(summary)
	if len(summ) > 80 {
		summ = summ[:80] + "..."
	}
	log.Printf("[OK] %s %s %s\n%#v", nom, title, date, summ)
	return err
}

func extractText(n *html.Node) string {
	var buf strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			buf.WriteString(c.Data + "")
		} else {
			buf.WriteString(extractText(c) + "")
		}
	}
	return strings.TrimSpace(buf.String())
}

func findNodeByClass(n *html.Node, className string) *html.Node {
	if n.Type == html.ElementNode && n.Data == "div" {
		for _, attr := range n.Attr {
			if attr.Key == "class" {
				classes := strings.Fields(attr.Val)
				for _, c := range classes {
					if c == className {
						return n
					}
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result := findNodeByClass(c, className)
		if result != nil {
			return result
		}
	}
	return nil
}
