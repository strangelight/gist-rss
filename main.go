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

	"github.com/gorilla/mux"
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

func serveError(c appengine.Context, w http.ResponseWriter, err error, msg string) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if msg == "" {
		io.WriteString(w, "Internal Server Error")
	} else {
		io.WriteString(w, msg)
	}
	c.Errorf("%v", err)
}

func handle(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	vars := mux.Vars(r)
	user := vars["user"]
	url := "https://api.github.com/users/" + user + "/gists"
	client := urlfetch.Client(c)
	res, err := client.Get(url)
	if err != nil {
		serveError(c, w, err, "")
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		serveError(c, w, err, "")
	}
	var data []Gist
	var gistitem *memcache.Item
	gistitem = &memcache.Item{
		Key:   "gist_" + user,
		Value: body,
	}
	if res.Header.Get("X-RateLimit-Remaining") != "0" {
		if err := memcache.Add(c, gistitem); err == memcache.ErrNotStored {
			err = memcache.Set(c, gistitem)
		}
		if err != nil {
			serveError(c, w, err, "")
		}
		err = json.Unmarshal(body, &data)
		if err != nil {
			serveError(c, w, err, "")
		}
	} else {
		if gistitem, err = memcache.Get(c, "gist_"+user); err == memcache.ErrCacheMiss {
			serveError(c, w, err, "Github API rate limit exceeded =/, back later...")
		} else if err != nil {
			serveError(c, w, err, "")
		}
	}
	err = json.Unmarshal(gistitem.Value, &data)
	if err != nil {
		serveError(c, w, err, "")
	}
	entries := make([]Entry, 0)
	for _, gist := range data {
		if gist.Description != "" {
			t, _ := time.Parse(time.RFC3339, gist.Updated_At)
			entries = append(entries, Entry{Title: gist.Description, Updated: t.String(), Id: gist.Html_Url, Type: "html", Content: "", Link: Link{Href: gist.Html_Url}})
		}
	}
	v := &Atom{Xmlns: "http://www.w3.org/2005/Atom", Title: user + " gists", Updated: time.Now().Format(time.RFC3339), Id: "http://gist-rss.appspot.com/" + user, Name: user, Email: "", Link: []Link{{"http://gist-rss.appspot.com/" + user, "self"}, {Href: "http://gist-rss.appspot.com/" + user}}, Entry: entries}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	enc := xml.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		serveError(c, w, err, "")
	}
}

func init() {
	m := mux.NewRouter()
	m.HandleFunc("/{user}", handle).Methods("GET")
	http.Handle("/", m)
}
