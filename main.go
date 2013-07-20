package main

import (
  //"encoding/json"
  "log"
  "net/url"
  "net/http"
  "io/ioutil"
  "html/template"
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
  
  err := http.ListenAndServe(":9999", nil)
  log.Println("Server started: listening on 9999")

  if err != nil {
    log.Fatal(err)
  }
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
  content, err := ioutil.ReadAll(resp.Body)
  token := string(content)
  log.Println("received access token: ", token)
}


type Client struct {
  client        *http.Client
  user_agent    string
  access_token  string
}

// {
//     "access_token": "23568145.222c75b.09b3dfca7d1c4dbc8ee0259a3b4ce41e",
//     "user": {
//         "username": "dvliman",
//         "bio": "all pictures are taken with iphone5",
//         "website": "http://davidliman.com",
//         "profile_picture": "http://images.ak.instagram.com/profiles/profile_23568145_75sq_1372927594.jpg",
//         "full_name": "David Liman",
//         "id": "23568145"
//     }
// }

// http://instagram.com/developer/endpoints/users/#get_users_feed_liked
//   access_token :  a valid access token.
//   count        :  count of media to return.
//   max_like_id  :  return media liked before this id
func (c *Client) user_liked_media() {
  // likes = "/api.instagram.com/v1/users/self/media/liked"
  // url := fmt.Sprintf(likes + "?access_token=%s", token)

  // resp, err := http.Get(url)


  // defer resp.Body.Close()

  // r := new(response)
  // err = json.NewDecoder(resp.Body).Decode(r)
}

func (c *Client) fetch(url string) {

}
