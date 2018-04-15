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

	"github.com/stretchr/testify/assert"
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

	t.Run("UploadSuccessWithoutProgressHandler", func(t *testing.T) {
		photo, err := client.Upload(sampleFile, nil)
		if err != nil {
			t.Fatal(err)
		}

		assert.NotEmpty(t, photo.ID)
		assert.NotEmpty(t, photo.AlbumID)
		assert.NotEmpty(t, photo.URL)
		assert.NotEmpty(t, photo.Name)
	})

	t.Run("UploadSuccessWithProgressHandler", func(t *testing.T) {

		var current int64
		var total int64
		progressHandler := func(c int64, t int64) {
			//fmt.Printf("current %d , total %d ", current, total)
			// I'm lazy to write this test. But it's work
			current = c
			total = t
		}

		photo, err := client.Upload(sampleFile, progressHandler)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, current, total)
		assert.NotEmpty(t, photo.ID)
		assert.NotEmpty(t, photo.AlbumID)
		assert.NotEmpty(t, photo.URL)
		assert.NotEmpty(t, photo.Name)
	})

}

func BenchmarkReUpload(b *testing.B) {
	for n := 0; n < b.N; n++ {
		sampleFile := GenNewSampleFile("./sample_data/sample.mp4")
		defer os.Remove(sampleFile)

		client := NewClient(GetTestCookies()...)

		if _, err := client.Upload(sampleFile, nil); err != nil {
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
