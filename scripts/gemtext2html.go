package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	togglePre := false
	inList := false
	for scanner.Scan() {
		line := scanner.Text()
		if togglePre {
			if line == "```" {
				fmt.Println("</pre>")
				togglePre = !togglePre
			} else {
				fmt.Println(line)
			}
			continue
		}

		if line == "" {
			if inList {
				inList = false
				fmt.Println("</ul>")
			}
			fmt.Println("<br>")
		} else if strings.HasPrefix(line, "###") {
			fmt.Printf("<h3>%s</h3>\n", strings.Trim(line[3:], " \t"))
		} else if strings.HasPrefix(line, "##") {
			fmt.Printf("<h2>%s</h2>\n", strings.Trim(line[2:], " \t"))
		} else if strings.HasPrefix(line, "#") {
			fmt.Printf("<h1>%s</h1>\n", strings.Trim(line[1:], " \t"))
		} else if strings.HasPrefix(line, "=>") {
			url, text, _ := strings.Cut(strings.Trim(line[2:], " \t"), " ")
			fmt.Printf("<a href=\"%s\">%s</a>\n", url, text)
		} else if strings.HasPrefix(line, ">") {
			fmt.Printf("<blockquote>%s</blockquote>\n", strings.Trim(line[1:], " \t"))
		} else if strings.HasPrefix(line, "*") {
			if !inList {
				inList = true
				fmt.Println("<ul>")
			}
			fmt.Printf("\t<li>%s</li>\n", strings.Trim(line[1:], " \t"))
		} else if line == "```" {
			togglePre = !togglePre
			fmt.Println("<pre>")
		} else {
			fmt.Printf("<p>%s</p>\n", line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error:", err)
	}
}
