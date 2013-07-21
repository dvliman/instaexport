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
)

func main() {
  http.HandleFunc("/", root)
  http.HandleFunc("/callback", callback)
  log.Fatal(http.ListenAndServe(":9999", nil))
}

func root(w http.ResponseWriter, r *http.Request) {
  t, _ := template.ParseFiles("index.html")
  var p struct{}
  
  if err := t.Execute(w, p); err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }
}

// oauth dance: http://instagram.com/developer/authentication/
func callback(w http.ResponseWriter, r *http.Request) {
  var qs = r.URL.Query()
  var code = qs.Get("code")

  payload := url.Values{}
  payload.Set("client_id", client_id)
  payload.Set("client_secret", client_secret)
  payload.Set("grant_type", "authorization_code")
  payload.Set("redirect_uri", redirect_url)
  payload.Set("code", code)
  
  log.Println("exchanging authorization code: ", code)

  resp, err := http.PostForm(access_token_url, payload)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
  }

  defer resp.Body.Close()

  stream, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
  }

  var oauth Token
  json.Unmarshal(stream, &oauth)

  access_token := oauth.AccessToken
  full_name    := oauth.User.FullName

  log.Println("access token: ", access_token)
  log.Println("full name: ", full_name)
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
