package main

import (
  "os"
  "io"
  "fmt"
  "log"
  "sync"
  "strings"
  "net/url"
  "net/http"
  "io/ioutil"
  "encoding/json"
  "path/filepath"
  "html/template"
)

const (
  client_id     = "222c75b62b6c4a0b8b789cbaebf75375"
  client_secret = "589eaa6bc7704eb7add52fcd229c463e"
  redirect_url  = "http://localhost:9999/callback"

  access_token_url = "https://api.instagram.com/oauth/access_token"
  media_liked_url  = "https://api.instagram.com/v1/users/self/media/liked"
)

// http handler func that can catch app specific error
// To be used with http.Handle instead of http.HandleFunc
type Handler func(http.ResponseWriter, *http.Request) *CustomError

func (fn Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  if e := fn(w, r); e != nil {
    log.Printf("%v", e.Error)
    http.Error(w, e.Message, e.Code)
  }
}

// custom error type that allows us to
// record the full error in the log and
// at the same time, display nice message to the user
type CustomError struct {
  Error   error
  Message string
  Code    int
}

// convenient func to deserialize json out of an http response
// may not be the best approach. interface{} is too generalized
// example use case:
//   var u User
//   entity(response, &u)
func entity(r *http.Response, v interface{}) error {
  defer r.Body.Close()
  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    return err
  }
  return json.Unmarshal(body, v)
}

func main() {
  http.Handle("/", Handler(root))
  http.Handle("/status", Handler(status))
  http.Handle("/callback", Handler(callback))
  log.Fatal(http.ListenAndServe(":9999", nil))
}

func root(w http.ResponseWriter, r *http.Request) *CustomError {
  t, _ := template.ParseFiles("index.html")
  t.Execute(w, nil)
  return nil
}

func status(w http.ResponseWriter, r *http.Request) *CustomError {

  return nil
}

// oauth dance: http://instagram.com/developer/authentication/
func callback(w http.ResponseWriter, r *http.Request) *CustomError {
  var qs = r.URL.Query()
  var code = qs.Get("code")
  log.Println("authorization code: ", code)

  payload := url.Values{}
  payload.Set("client_id", client_id)
  payload.Set("client_secret", client_secret)
  payload.Set("grant_type", "authorization_code")
  payload.Set("redirect_uri", redirect_url)
  payload.Set("code", code)

  resp, err := http.PostForm(access_token_url, payload)
  if err != nil {
    return &CustomError{err, "Error getting access token", 500}
  }

  var oauth Token
  entity(resp, &oauth)

  process := NewProcess(oauth)
  process.start()

  root(w, r)
  return nil
}

type Token struct {
  AccessToken string `json:"access_token"`
  User        struct {
    Username       string `json:"username"`
    Bio            string `json:"bio"`
    Website        string `json:"website"`
    ProfilePicture string `json:"profile_picture"`
    FullName       string `json:"full_name"`
    Id             string `json:"id"`
  }
}

// These are not full reflection of Instagram APIs
// They are only subset of json that I care
type APIResponse struct {
  Pagination Pagination `json:"pagination"`
  Meta       Meta       `json:"meta"`
  Data       []Data     `json:"data"`
}

type Pagination struct {
  NextUrl       *string `json:"next_url"`
  NextMaxLikeId *string `json:"next_max_like_id"`
}

type Meta struct {
  Code         int    `json:"code"`
  ErrorType    string `json:"error_type"`
  ErrorMessage string `json:"error_message"`
}

type Data struct {
  Images Images `json:"images"`
}

type Images struct {
  StandardResolution Resolution `json:"standard_resolution"`
}

type Resolution struct {
  Url string
}

// a unit of work per user identified by access token
// methods should start in a goroutine and report to caller
// used to check when the process is complete
type Process struct {
  user  string
  token string

  path  string
  urls  []string
  done  chan int
}

func NewProcess(oauth Token) *Process {
  return &Process {
    user : oauth.User.Username,
    token: oauth.AccessToken,

    path : "",
    urls : make([]string, 0),
    done : make(chan int),
  }
}

func (p *Process) prepare() {
  pwd, _ := os.Getwd()
  p.path = filepath.Join(pwd, "/tmp/", p.token)
  os.MkdirAll(p.path, 077)
}

// http://instagram.com/developer/endpoints/users/#get_users_feed_liked
func (p *Process) fetch(endpoint string) {
  if endpoint == "" {
    endpoint = fmt.Sprintf(media_liked_url+"?access_token=%s", p.token)
  }

  log.Println("fetching: " + endpoint)
  resp, err := http.Get(endpoint)
  if err != nil {
    log.Println(err)
  }

  var api APIResponse
  entity(resp, &api)

  for _, like := range api.Data {
    p.urls = append(p.urls, like.Images.StandardResolution.Url)
  }

  // follow through if there are more user's liked media
  if api.Pagination.NextUrl != nil {
    p.fetch(*api.Pagination.NextUrl)
  }
}

func (p *Process) download(url string) {
  parts := strings.Split(url, "/")
  name  := parts[len(parts) - 1]

  file, err := ioutil.TempFile(p.path, name)
  if err != nil {
    log.Println(err)
    return
  }

  defer file.Close()

  resp, err := http.Get(url)
  if err != nil {
    log.Println(err)
    return
  }

  defer resp.Body.Close()
  io.Copy(file, resp.Body)
}

func (p *Process) zip() {
  fmt.Println("zipping")
}

func (p *Process) start() {
  p.prepare()
  p.fetch("")

  var wg sync.WaitGroup
  for _, url := range p.urls {
    wg.Add(1)
    go p.download(url)
    wg.Done()
  }

  // blocks until all downloads are done (in parallel)
  // when finish, start zipping
  wg.Wait()
  p.zip()
}