package main

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"appengine"
	"appengine/memcache"
	"appengine/urlfetch"
)

const (
	APPENGINE_ID = "gist-rss"
	GITHUB_ID    = "danielvargas"
	EMAIL        = "danielgvargas@gmail.com"
)

type Gist struct {
	Url          string
	Forks_Url    string
	Commits_Url  string
	Id           string
	Git_Pull_Url string
	Git_Push_Url string
	Html_Url     string
	Public       bool
	Created_At   string
	Updated_At   string
	Description  string
	Comments     int
	Comments_Url string
	Files        map[GistFilename]File `json:"files,omitempty"`
	Users        map[GistLogin]User    `json: "users,omitempty"`
}

type GistFilename string

type File struct {
	Filename GistFilename
	Type     string
	Language string
	Raw_Url  string
	Size     int
}

type GistLogin string

type User struct {
	Login               GistLogin
	Id                  string
	Avatar_Url          string
	Gravatar_Url        string
	Url                 string
	Html_Url            string
	Followers_Url       string
	Following_Url       string
	Gists_Url           string
	Starred_Url         string
	Subscriptions_Url   string
	Organizations_Url   string
	Repos_Url           string
	Events_Url          string
	Received_Events_Url string
	Type                string
}

type Link struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
}

type Atom struct {
	XMLName xml.Name `xml:"feed"`
	Xmlns   string   `xml:"xmlns,attr"`
	Title   string   `xml:"title"`
	Link    []Link   `xml:"link"`
	Updated string   `xml:"updated"`
	Id      string   `xml:"id"`
	Name    string   `xml:"author>name"`
	Email   string   `xml:"author>email"`
	Entry   []Entry  `xml:"entry"`
}

type Entry struct {
	Title   string `xml:"title"`
	Link    Link   `xml:"link"`
	Updated string `xml:"updated"`
	Id      string `xml:"id"`
	Content string `xml:"content"`
	Type    string `xml:"type,attr"`
}

func serveError(c appengine.Context, w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, "Internal Server Error")
	c.Errorf("%v", err)
}

func handle(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	url := "https://api.github.com/users/" + GITHUB_ID + "/gists"
	client := urlfetch.Client(c)
	res, _ := client.Get(url)
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	var data []Gist
	if res.Header.Get("X-RateLimit-Remaining") != "0" {
		json.Unmarshal(body, &data)
	} else if res.Header.Get("X-RateLimit-Remaining") == "1" {
		gistitem := &memcache.Item{
			Key:   "gist",
			Value: body,
		}
		memcache.Set(c, gistitem)
		json.Unmarshal(body, &data)
	} else {
		gistitem, _ := memcache.Get(c, "gist")
		json.Unmarshal(gistitem.Value, &data)
	}
	entries := make([]Entry, 0)
	for _, gist := range data {
		if gist.Description != "" {
			t, _ := time.Parse(time.RFC3339, gist.Updated_At)
			entries = append(entries, Entry{Title: gist.Description, Updated: t.String(), Id: gist.Html_Url, Content: "<script src='https://gist.github.com/" + GITHUB_ID + "/" + gist.Id + ".js'></script>", Type: "html", Link: Link{Href: gist.Html_Url}})
		}
	}

	v := &Atom{Xmlns: "http://www.w3.org/2005/Atom", Title: GITHUB_ID + " gists", Updated: time.Now().Format(time.RFC3339), Id: "http://" + APPENGINE_ID + ".appspot.com", Name: GITHUB_ID, Email: EMAIL, Link: []Link{{"http://" + APPENGINE_ID + ".appspot.com", "self"}, {Href: "http://" + APPENGINE_ID + ".appspot.com"}}, Entry: entries}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	enc := xml.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		serveError(c, w, err)
	}
}

func init() {
	http.HandleFunc("/", handle)
}
