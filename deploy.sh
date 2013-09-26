GOOS=darwin GOARCH=amd64 go build -o instaexport.osx *.go
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -0 instaexport.linux *.go
