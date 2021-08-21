package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/canhlinh/gphoto"
)

func main() {

	cookies := GetCookiesFromJSON("./cookie.json")
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
	// fmt.Printf("current %d , total %d ", current, total)
}
