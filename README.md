# GPHOTO
A small lib to upload files to the google photo

[![Build Status](https://circleci.com/gh/canhlinh/gphoto.svg?style=svg)](https://circleci.com/gh/canhlinh/gphoto)
[![GoDoc](https://godoc.org/github.com/canhlinh/gphoto?status.svg)](http://godoc.org/github.com/canhlinh/gphoto)

# Features
- Uploads file to google photo account via user's cookies, via user's credential (user, pass).
- Update upload's progress while a file is uploading.

# Getting Started

If you want to login to google automaticaly, you have to install ChromeDriver firstly.
See more about ChromeDrive at https://sites.google.com/a/chromium.org/chromedriver/

I'm not ensure the code can work smoothy. Use at your own risk.
July 14 2018, This code still can work well.

You can take a look at the test code to get some example.

## Install
```
go get -u github.com/canhlinh/gphoto
```

## Quick Start
```
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/canhlinh/gphoto"
)

func main() {

	cookies := GetCookiesFromJSON("./json.")
	client := gphoto.NewClient(cookies...)

	photo, err := client.Upload("../sample_data/sample.mp4", "sample.mp4", "AnyAlbumName", progressHandler)
	if err != nil {
		panic(err)
	}
	fmt.Println(photo)
}

// GetCookiesFromJSON parse cookies from a JSON file
// The JSON file can be exported by this extension https://chrome.google.com/webstore/detail/editthiscookie/fngmhnnpilhplaeedifhccceomclgfbg
func GetCookiesFromJSON(path string) []*http.Cookie {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	var cookies []*http.Cookie
	json.NewDecoder(file).Decode(&cookies)
	return cookies
}

func progressHandler(current int64, total int64) {
	fmt.Printf("current %d , total %d ", current, total)
}
```

## Run test

Exports your google photo into an variable GPHOTO_COOKIES_BASE64.
Run test:
```
go test -v -race
```
