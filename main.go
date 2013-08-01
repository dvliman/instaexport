package main

import (
  "os"
  "io"
  "fmt"
  "log"
  //"sync"
  "syscall"
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
  download_path = "/tmp/instaexport/"

  access_token_url = "https://api.instagram.com/oauth/access_token"
  media_liked_url  = "https://api.instagram.com/v1/users/self/media/liked"
)

// higher order function for http handler that can catch error
type Handler func(http.ResponseWriter, *http.Request) *CustomError

func (fn Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  if e := fn(w, r); e != nil {
    log.Printf("%v", e.Error)
    http.Error(w, e.Message, e.Code)
  }
}

// to record full error and at the same time
// present nicer messsage to the end user
type CustomError struct {
  Error   error
  Message string
  Code    int
}

// deserialize json out of an http response
// use case: var u User; entity(response, &u)
func entity(r *http.Response, v interface{}) error {
  defer r.Body.Close()
  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    return err
  }
  return json.Unmarshal(body, v)
}

// http://en.wikipedia.org/wiki/Umask
func MkdirAll(location string) {
  oldMask := syscall.Umask(0)
  err := os.MkdirAll(location, os.ModePerm)
  if err != nil {
    log.Fatal(err)
  }
  syscall.Umask(oldMask)
}

func main() {
  MkdirAll(download_path)

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

  log.Printf("code: %s, token: %s, name: %s\n", code, oauth.AccessToken, oauth.User.Username)
  process := NewProcess(oauth)
  start(process)

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

// not full reflection of Instagram APIs
// only subset of json that I care
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

type Process struct {
  user  string
  token string

  lastFetched string
  urls        []string
  done        chan int
}

var state = map[string] Process {}

func NewProcess(oauth Token) *Process {
  return &Process {
    user : oauth.User.Username,
    token: oauth.AccessToken,

    lastFetched: "",
    urls : make([]string, 0),
    done : make(chan int),
  }
}

// http://instagram.com/developer/endpoints/users/#get_users_feed_liked
func fetch (p *Process) {
  if p.lastFetched == "" {
    p.lastFetched = fmt.Sprintf(media_liked_url+"?access_token=%s", p.token)
  }

  resp, err := http.Get(p.lastFetched)
  if err != nil {
    log.Println(err)
  }

  var api APIResponse
  entity(resp, &api)

  // for the sake of logging purpose
  if api.Pagination.NextMaxLikeId != nil {
    log.Println("fetching:", *api.Pagination.NextMaxLikeId)
  }

  for _, like := range api.Data {
    p.urls = append(p.urls, like.Images.StandardResolution.Url)
  }

  // follow through if there are more user's liked media
  if api.Pagination.NextUrl != nil {
    p.lastFetched = *api.Pagination.NextUrl
    fetch(p)
  }
}

func download(src, dest string) {
  parts := strings.Split(src, "/")
  name  := parts[len(parts) - 1]
  destination := filepath.Join(dest, name)

  file, err := os.OpenFile(destination, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
  if err != nil {
    log.Println("1")
    log.Println(err)
    return
  }

  defer file.Close()

  response, err := http.Get(src)
  if err != nil {
    log.Println("2")
    log.Println(err)
    return
  }

  defer response.Body.Close()
  io.Copy(file, response.Body)
}

func start (p *Process) {
  target := filepath.Join(download_path, p.token)
  MkdirAll(target)
  fetch(p)

  log.Println("destination: ", target)
  log.Println("image count: ", len(p.urls))

  //var wg sync.WaitGroup
  for _, url := range p.urls {
    //wg.Add(1)
    download(url, target)
    //wg.Done()
  }

  //wg.Wait()
}