package gphoto

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"testing"
	"time"
)

const (
	CookieJsonFile = "cookie.json"
)

func GetBinaryTime() []byte {
	return []byte(fmt.Sprintf("%d", time.Now().Unix()))
}

func GenNewSampleFile(orginalPath string) string {
	file, err := os.Open(orginalPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	stats, _ := file.Stat()

	bTime := GetBinaryTime()
	file.WriteAt(bTime, stats.Size()-int64(len(bTime)))

	filePath := path.Dir(orginalPath) + "/" + string(bTime) + "_" + path.Base(orginalPath)
	sampleFile, err := os.Create(filePath)
	io.Copy(sampleFile, file)
	return filePath
}

func GetTestCookies() []*http.Cookie {
	file, err := os.Open(CookieJsonFile)
	if err != nil {
		panic(err)
	}
	var cookies []*http.Cookie
	json.NewDecoder(file).Decode(&cookies)
	return cookies
}

func TestUpload(t *testing.T) {

	sampleFile := GenNewSampleFile("./sample_data/sample.mp4")
	defer os.Remove(sampleFile)

	client := NewClient(GetTestCookies()...)

	photo, err := client.Upload(sampleFile)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(photo.ID, photo.URL, photo.Name, photo.AlbumID)
}

func BenchmarkReUpload(b *testing.B) {
	for n := 0; n < b.N; n++ {
		sampleFile := GenNewSampleFile("./sample_data/sample.mp4")
		defer os.Remove(sampleFile)

		client := NewClient(GetTestCookies()...)

		if _, err := client.Upload(sampleFile); err != nil {
			b.Fatal(err)
		}
	}
}

func TestLogin(t *testing.T) {

	user := os.Getenv("GOOGLE_USERNAME")
	pass := os.Getenv("GOOGLE_PASSWORD")

	if len(user) == 0 || len(pass) == 0 {
		t.Fatal("User or passowrd is empty")
	}

	c := NewClient()
	if err := c.Login(user, pass); err != nil {
		t.Fatal(err)
	}
}
