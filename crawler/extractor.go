package crawler

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

type pageData struct {
	title       string
	description string
	body        string
	links       []string
}

// extract parses HTML and returns title, meta description, body text, and links
func extract(r io.Reader, baseURL string) pageData {
	doc, err := html.Parse(r)
	if err != nil {
		return pageData{}
	}

	var data pageData
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if n.FirstChild != nil {
					data.title = strings.TrimSpace(n.FirstChild.Data)
				}
				return
			case "meta":
				if attrVal(n, "name") == "description" {
					// take only the first line — multiline meta tags use newlines
					// as sentence separators; the first line is the primary description
					raw := strings.SplitN(attrVal(n, "content"), "\n", 2)[0]
					data.description = strings.TrimSpace(raw)
				}
				return
			case "script", "style", "noscript", "nav", "header", "footer", "aside":
				return // skip non-content subtrees
			case "a":
				if href := attrVal(n, "href"); href != "" {
					if resolved := resolveLink(href, baseURL); resolved != "" {
						data.links = append(data.links, resolved)
					}
				}
			}
		}

		if n.Type == html.TextNode && n.Parent != nil {
			switch n.Parent.Data {
			case "script", "style", "noscript":
				// already skipped above, but guard here too
			default:
				text := strings.TrimSpace(n.Data)
				if text != "" {
					data.body += text + " "
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	data.body = strings.TrimSpace(data.body)
	return data
}

// attrVal returns the value of the named attribute, or "".
func attrVal(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
