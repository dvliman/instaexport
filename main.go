package main

import (
	"archive/zip"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	client_id     = "222c75b62b6c4a0b8b789cbaebf75375"
	client_secret = "589eaa6bc7704eb7add52fcd229c463e"

	redirect_url  = "http://localhost:9999/callback"
	download_path = "/tmp/instaexport/"

	access_token_url = "https://api.instagram.com/oauth/access_token"
	media_liked_url  = "https://api.instagram.com/v1/users/self/media/liked"
)

// http handler type that can catch error
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
	body, _ := ioutil.ReadAll(r.Body)
	return json.Unmarshal(body, v)
}

func writeCookie(w http.ResponseWriter, oauth Token) {
	cookie := &http.Cookie{Name: "instaexport", Value: oauth.AccessToken}
	http.SetCookie(w, cookie)
}

// https://gist.github.com/mynameisfiber/2853066
func createHttpClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		// dial function for establishing TCP connections
		Dial: func(network, addr string) (net.Conn, error) {
			deadline := time.Now().Add(800 * time.Millisecond)
			c, err := net.DialTimeout(network, addr, time.Second)
			if err != nil {
				return nil, err
			}
			c.SetDeadline(deadline)
			return c, nil
		},
	}

	return &http.Client{
		Transport: transport,
		// redirect policy stop after being redirected once
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 1 {
				return errors.New("stop following redirect")
			}
			return nil
		},
	}
}

// this server uses combination of cookie and filesystem seek to keep track of export process
// server is stateless and can be launched as multi-processes backend upstream on a singlebox
// filesystem seek is not that expensive on ssd
func main() {
	log.Println("-- instaexport started")

	http.Handle("/", Handler(root))
	http.Handle("/export", Handler(export))
	http.Handle("/status", Handler(status))
	http.Handle("/callback", Handler(callback))
	log.Fatal(http.ListenAndServe(":9999", nil))
}

func root(w http.ResponseWriter, r *http.Request) *CustomError {
	t, _ := template.ParseFiles("index.html")
	t.Execute(w, nil)
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

	process := NewProcess(oauth)
	go run(process)

	writeCookie(w, oauth)
	root(w, r)
	return nil
}

func status(w http.ResponseWriter, r *http.Request) *CustomError {
	cookie, err := r.Cookie("instaexport")
	if err != nil {
		return &CustomError{err, "Can't read cookies. Did you disable it?", 500}
	}

	check := filepath.Join(download_path, cookie.Value + "-done")
	f, err := os.Stat(check)
	if f != nil { } // why I can't _ on os.FileInfo?
	if err == nil {
		fmt.Fprintf(w, "OK")
	}
	if os.IsNotExist(err) {
		fmt.Fprintln(w, "KO")
	}
	return nil
}

func export(w http.ResponseWriter, r *http.Request) *CustomError {
	cookie, err := r.Cookie("instaexport")
	if err != nil {
		return &CustomError{err, "Can't read cookies. Did you disable it?", 500}
	}

	w.Header().Set("Content-Disposition", "attachment; filename=instaexport.zip")
	w.Header().Set("Content-Type", "application/zip")
	w.WriteHeader(200)

	zw := zip.NewWriter(w)
	defer zw.Close()

	target := filepath.Join(download_path, cookie.Value)
	files, _ := ioutil.ReadDir(target)
	for _, f := range files {
		if err := archive(zw, f.Name()); err != nil {
			return &CustomError{err, "failed to zip. try again?", 500}
		}
	}

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
// only subset of json that I am interested with
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

	path string
	last string
	urls []string
}

func NewProcess(oauth Token) *Process {
	return &Process{
		user:  oauth.User.Username,
		token: oauth.AccessToken,

		path: "",
		last: "",
		urls: make([]string, 0),
	}
}

func run(p *Process) {
	log.Printf("name: %s, token: %s\n", p.user, p.token)
	prepare(p)
	fetch(p)
	report(p)
	download(p)
	done(p)
}

func prepare(p *Process) {
	p.path = filepath.Join(download_path, p.token)

	// http://en.wikipedia.org/wiki/Umask
	oldMask := syscall.Umask(0)
	os.MkdirAll(p.path, os.ModePerm)
	syscall.Umask(oldMask)
}

// http://instagram.com/developer/endpoints/users/#get_users_feed_liked
func fetch(p *Process) {
	if p.last == "" {
		p.last = fmt.Sprintf(media_liked_url+"?access_token=%s", p.token)
	}

	log.Println("fetching: ", p.last)
	resp, err := http.Get(p.last)
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
		p.last = *api.Pagination.NextUrl
		fetch(p)
	}
}

func report(p *Process) {
	log.Println("destination: ", p.path)
	log.Println("image count: ", len(p.urls))
}

// it is very cheap to create goroutines that
// we quickly run out of file descriptors.
// use rolling bucket to preserve some fd(s)
func download(p *Process) {
	var wg sync.WaitGroup
	wg.Add(len(p.urls))

	// prefill bucket with 100 tokens
	bucket := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		bucket <- true
	}

	// borrow one token each time we download. return when download is done
	// this way, we have a rolling bucket which prevent upper limit 100 concurrent reqs
	// if we run out of token, the method will block until it can borrow
	for i, url := range p.urls {
		go func(src, dest string) {

			<-bucket
			defer func() { bucket <- true }()
			grab(src, dest)
			wg.Done()

			/* rewrite the picture filename so its ordered */
		}(url, filepath.Join(p.path, strconv.Itoa(i)+".jpg"))
	}

	// block until all downloads are done
	// can't continue to the next stage of pipeline
	wg.Wait()
}

func done(p *Process) {
	//mark := filepath.Join(p.path, "-done")
	//os.OpenFile(mark, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
}

func kill(p *Process) {
	os.RemoveAll(p.path)
	p = nil
}

func grab(src, dest string) {
	file, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return
	}

	fmt.Println("downloading: ", src)
	
	httpClient := createHttpClient()
	request, err := http.NewRequest("GET", src, nil)
	request.Header.Add("User-Agent", "Instaexport -- export your liked pictures on instagram")
	
	response, err := httpClient.Do(request)
	if err != nil {
		return
	}

	defer file.Close()
	defer response.Body.Close()
	io.Copy(file, response.Body)
}

func archive(zipper *zip.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}

	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = filename
	writer, err := zipper.CreateHeader(header)
	if err != nil {
		return err
	}

	io.Copy(writer, file)
	return err
}