package main

import (
  "log"
  "net/url"
  "net/http"
  "io/ioutil"
  "html/template"
  "encoding/json"
)

const (
  client_id        = "222c75b62b6c4a0b8b789cbaebf75375"
  client_secret    = "589eaa6bc7704eb7add52fcd229c463e"
  redirect_url     = "http://localhost:9999/callback"

  access_token_url = "https://api.instagram.com/oauth/access_token"
  media_liked_url  = "https://api.instagram.com/v1/users/self/media/liked"
)

func main() {
  http.Handle("/", safe(root))
  http.Handle("/callback", safe(callback))
  log.Fatal(http.ListenAndServe(":9999", nil))
}

// a "safe" http.Handle that can catch app specific error
// To be used with http.Handle instead of http.HandleFunc
type safe func(http.ResponseWriter, *http.Request) *CustomError

func (fn safe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func root(w http.ResponseWriter, r *http.Request) *CustomError{
  t, _ := template.ParseFiles("index.html")

  if err := t.Execute(w, nil); err != nil {
    return &CustomError{err, "Could not parse template", 500}
  }
  return nil
}

// oauth dance: http://instagram.com/developer/authentication/
func callback(w http.ResponseWriter, r *http.Request) *CustomError{
  var qs = r.URL.Query()
  var code = qs.Get("code")

  payload := url.Values{}
  payload.Set("client_id", client_id)
  payload.Set("client_secret", client_secret)
  payload.Set("grant_type", "authorization_code")
  payload.Set("redirect_uri", redirect_url)
  payload.Set("code", code)
  
  log.Println("authorization code: ", code)

  resp, err := http.PostForm(access_token_url, payload)
  if err != nil {
    return &CustomError{err, "Could not get access token from Instagram", 500}
  }

  defer resp.Body.Close()

  stream, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return &CustomError{err, "Could not parse response received from Instagram", 500}
  }

  var oauth Token
  json.Unmarshal(stream, &oauth)

  access_token := oauth.AccessToken
  full_name    := oauth.User.FullName

  log.Println("access token: ", access_token)
  log.Println("full name: ", full_name)
  return nil
}

type Token struct {
  AccessToken       string  `json:"access_token"`
  User struct {
    Username        string  `json:"username"`
    Bio             string  `json:"bio"`
    Website         string  `json:"website"`
    ProfilePicture  string  `json:"profile_picture"`
    FullName        string  `json:"full_name"`
    Id              string  `json:"id"`
  }
}

type APIResponse struct {
  Pagination Pagination
  Meta       Meta
  Data       []Data
}

type Pagination struct {
  NextURL        string  `json:"next_url"`
  NextMaxLikeId  string  `json:"next_max_like_id"`
}

type Meta struct {
  Code          string  `json:"code"`
  ErrorType     string  `json:"error_type,omitempty"`
  ErrorMessage  string  `json:"error_message,omitempty"`
}

type Data struct {
  //Images Images   `json:"images"`
}

// http://instagram.com/developer/endpoints/users/#get_users_feed_liked
//   access_token :  a valid access token.
//   count        :  count of media to return.
//   max_like_id  :  return media liked before this id
// func (c *Client) user_liked_media() {
//   likes = "/api.instagram.com/v1/users/self/media/liked"
//   url := fmt.Sprintf(likes + "?access_token=%s", token)

//   resp, err := http.Get(url)


//   defer resp.Body.Close()

//   r := new(response)
//   err = json.NewDecoder(resp.Body).Decode(r)
// }

// func (c *Client) fetch(url string) {

// }
