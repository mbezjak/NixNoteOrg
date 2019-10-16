package main

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/html"
	"strconv"
)

type Query struct {
	Notes []Note `xml:"Note"`
}

type Node struct {
	Token html.Token
	Text  string
}

type Nodes []Node

type Note struct {
	Guid       string   `xml:"Guid"`
	Title      string   `xml:"Title"`
	Content    string   `xml:"Content"`
	Created    string   `xml:"Created"`
	Tags       []string `xml:"Tag"`
	Attributes struct {
		Author    string  `xml:"Author"`
		Latitude  float64 `xml:"Latitude"`
		Longitude float64 `xml:"Longitude"`
		Source    string `xml:"Source"`
		SourceUrl string `xml:"SourceUrl"`
	} `xml:"Attributes"`

	Resources []Resource `xml:"NoteResource"`
}

type Resource struct {
	Mime string `xml:"Mime"`
	Data struct {
		Content  string `xml:"Body"`
		Hash string `xml:"BodyHash"`
		Encoding string `xml:"encoding,attr"`
	} `xml:"Data"`
	ResourceAttributes struct {
		FileName string `xml:"FileName"`
	} `xml:"ResourceAttributes"`
}

var (
	readFile       = ""
	attachmentPath = ""
	attachments    = make(map[string]string)
)

func getAttr(attribute string, token html.Token) string {
	for _, attr := range token.Attr {
		if attr.Key == attribute {
			return attr.Val
		}
	}
	return ""
}

func (nodes Nodes) orgFormat() string {
	var value strings.Builder
	header := 0
	table := 0
	insidePre := false
	list := 0
	listValue := []int{}

	for _, node := range nodes {
		switch node.Token.Type {
		case html.SelfClosingTagToken:
			switch node.Token.Data {
			case "en-note":
				break

			case "en-media":
				value.WriteString("\n")
				value.WriteString("[[./" + filepath.Base(attachmentPath) + "/")
				value.WriteString(attachments[getAttr("hash", node.Token)] + "]]")

			case "en-todo":
				switch getAttr("checked", node.Token) {
				case "true":
					value.WriteString("\n- [X] ")
				case "false":
					value.WriteString("\n- [ ] ")
				}

			case "br":
				value.WriteString("\n")

			case "hr":
				value.WriteString("\n------------------------------------\n")
			}

		case html.StartTagToken:
			switch node.Token.Data {

			case "a":
				// We do not want links in the header
				if header == 0 {
					value.WriteString("[[" + getAttr("href", node.Token) + "][")
				}
			case "p", "div":
				value.WriteString("\n")
			case "u":
				value.WriteString("_")
			case "i":
				value.WriteString("/")
			case "b", "strong", "em":
				value.WriteString("*")
			case "del":
				value.WriteString("+")
			case "h1":
				value.WriteString("\n** ")
				header++
			case "h2":
				value.WriteString("\n*** ")
				header++
			case "h3":
				value.WriteString("\n**** ")
				header++
			case "h4":
				value.WriteString("\n***** ")
				header++
			case "h5":
				value.WriteString("\n****** ")
				header++
			case "h6":
				value.WriteString("\n******* ")
				header++

				// These tags are ignored
			case "en-note", "span", "tr", "tbody", "abbr", "th", "thead", "ins", "img":
				break
			case "sup", "sub", "small", "br", "dl", "dd", "dt", "font", "colgroup", "cite":
				break
			case "address", "s", "map", "area", "center", "q":
				break

			case "hr":
				value.WriteString("\n------------------------------------\n")
			case "en-media":
				value.WriteString("\n")
				value.WriteString("[[./" + filepath.Base(attachmentPath) + "/")
				value.WriteString(attachments[getAttr("hash", node.Token)] + "]]")
			case "table":
				table++
			case "td":
				value.WriteString("|")
			case "ol":
				list++
				listValue = append(listValue, 1)
			case "ul":
				list++
				listValue = append(listValue, 0)
			case "li":
				value.WriteString("\n")
				for i := 0; i <= list; i++ {
					value.WriteString("  ")
				}
				if list > 0 {
					switch listValue[list-1] {
					case 0:
						value.WriteString("- ")
					default:
						value.WriteString(fmt.Sprintf("%d.", listValue[list-1]))
						listValue[list-1] = listValue[list-1] + 1
					}
				}
			case "code", "tt", "kbd":
				if !insidePre {
					value.WriteString("~")
				}
			case "pre":
				value.WriteString("\n#+BEGIN_SRC\n")
				insidePre = true
			case "blockquote":
				value.WriteString("\n#+BEGIN_QUOTE\n")

			default:
				fmt.Println("skip token: " + node.Token.Data)
			}

		case html.EndTagToken:
			switch node.Token.Data {
			case "u":
				value.WriteString("_")
			case "i":
				value.WriteString("/")
			case "b", "strong", "em":
				value.WriteString("*")
			case "del":
				value.WriteString("+")
			case "a":
				if header == 0 {
					value.WriteString("]]")
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				header--
			case "table":
				table--
			case "tr":
				value.WriteString("|\n")
			case "ol", "ul":
				list--
				listValue = listValue[:len(listValue)-1]
			case "code", "tt", "kbd":
				if !insidePre {
					value.WriteString("~")
				}
			case "pre":
				value.WriteString("\n#+END_SRC\n")
				insidePre = false
			case "blockquote":
				value.WriteString("\n#+END_QUOTE\n")
			}

		case html.TextToken:
			value.WriteString(html.UnescapeString(node.Token.String()))
		}
	}

	return value.String()
}

func parseHTML(r io.Reader) Nodes {
	var nodes Nodes
	d := html.NewTokenizer(r)
	for {
		tokenType := d.Next()
		if tokenType == html.ErrorToken {
			return nodes
		}

		token := d.Token()
		nodes = append(nodes, Node{token, ""})
	}
}

func main() {
	wordPtr := flag.String("input", "enex File", "relative path to enex file")
	flag.Parse()
	if wordPtr == nil || *wordPtr == "" {
		panic("input file is missing")
	}
	fmt.Println("input:", *wordPtr)

	// Open the file given at commandline
	readFile = *wordPtr
	xmlFile, err := os.Open(readFile)

	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}

	defer func() { _ = xmlFile.Close() }()

	currentDir, fileName := filepath.Split(readFile)
	ext := filepath.Ext(readFile)
	baseName := strings.TrimSuffix(fileName, ext)

	newWrittenDir := filepath.Join(currentDir, baseName)
	if _, err = os.Stat(newWrittenDir); os.IsNotExist(err) {
		_ = os.Mkdir(newWrittenDir, 0711)
	}

	b, _ := ioutil.ReadAll(xmlFile)

	var q Query
	_ = xml.Unmarshal(b, &q)

	attachmentsCount := 0
	notesCount := 0

	for _, note := range q.Notes {
		cdata := []byte(note.Content)
		reader := bytes.NewReader(cdata)
		nodes := parseHTML(reader)

		if len(note.Resources) != 0 {
			// Create Attachments Directory if not exists
			attachmentPath = filepath.Join(newWrittenDir, sanitize(note.Title))
			if _, err = os.Stat(attachmentPath); os.IsNotExist(err) {
				_ = os.Mkdir(attachmentPath, 0711)
			}

			for _, attachment := range note.Resources {
				if attachment.ResourceAttributes.FileName == "" {
					attachment.ResourceAttributes.FileName = attachment.Data.Hash
				}
				attachments[attachment.Data.Hash] = attachment.ResourceAttributes.FileName
				sDec, _ := hex.DecodeString(attachment.Data.Content)
				err := ioutil.WriteFile(attachmentPath+"/"+attachment.ResourceAttributes.FileName, sDec, 0644)
				if err != nil { panic(err) }
				attachmentsCount++
			}
		}

		noteFileName := sanitize(note.Title) + ".org"
		newFile, err := os.Create(filepath.Join(newWrittenDir, noteFileName))
		if err != nil { panic(err) }
		_, _ = newFile.WriteString(note.orgProperties())
		_, _ = newFile.WriteString(nodes.orgFormat())
		_ = newFile.Close()

		notesCount++
	}

	fmt.Printf("\nThere are %d notes and %d attachments created", notesCount, attachmentsCount)
}

func (note Note) orgProperties() string {
	var result strings.Builder
	attr := note.Attributes

	result.WriteString("#+TITLE: " + note.Title + "\n")
	result.WriteString("#+STARTUP: showall" + "\n")

	if attr.Author != "" {
		result.WriteString("#+AUTHOR: " + attr.Author + "\n")
	}
	if len(note.Tags) > 0 {
		result.WriteString("#+TAGS: ")
		result.WriteString(strings.Join(note.Tags, " ") + "\n")
	}
	if note.Created != "" {
		c, err := strconv.ParseInt(note.Created, 10, 64)
		if err != nil { panic(err) }
		result.WriteString("#+DATE: " + time.Unix(c / 1000, 0).String() + "\n")
	}
	if attr.Latitude > 0 {
		result.WriteString(fmt.Sprintf("#+LAT: %f\n", attr.Latitude))
		result.WriteString(fmt.Sprintf("#+LON: %f\n", attr.Longitude))
	}
	if attr.Source != "" {
		result.WriteString("#+SOURCE: " + attr.Source + "\n")
	}
	if attr.SourceUrl != "" {
		result.WriteString("#+DESCRIPTION: " + attr.SourceUrl + "\n")
	}
	if note.Guid != "" {
		// This is very hacky. This link works for now (and for me). Those
		// hardcoded parameters are not documented anywhere so I don't know for
		// how long this could work or if it'll work in any situation.
		// There is a "copy link" in evernote web, but the needed info is not
		// part of nnex files so we cannot produce it.
		// Note: evernote will correct query parameter `s` with correct value.
		result.WriteString("#+EVERNOTE_URL: https://evernote.com/Home.action#n=" +
			note.Guid +
			"&s=s1&ses=4&sh=2\n")
	}

	return result.String()
}

func sanitize(title string) string {
	title = strings.TrimSpace(strings.ToLower(title))
	title = strings.Replace(title, "-", "", -1)
	title = strings.Replace(title, "'", "", -1)
	title = strings.Replace(title, "(", "", -1)
	title = strings.Replace(title, ")", "", -1)
	title = strings.Replace(title, ",", "", -1)
	title = strings.Replace(title, ":", "", -1)
	title = strings.Replace(title, "|", "", -1)
	title = strings.Replace(title, "?", "", -1)
	title = strings.Replace(title, ".", "", -1)
	title = strings.Replace(title, "/", "", -1)
	title = strings.Replace(title, "\"", "", -1)
	title = strings.Replace(title, " ", "-", -1)
	return title
}
