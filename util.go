package gphoto

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

//NewJSONBody create a new json request body from an interface
func NewJSONBody(model interface{}) io.Reader {
	var buf = &bytes.Buffer{}
	json.NewEncoder(buf).Encode(model)
	return buf
}

func BodyToString(body io.Reader) string {
	var buf bytes.Buffer
	io.Copy(&buf, body)
	return buf.String()
}

func BodyToBytes(body io.Reader) []byte {
	var buf bytes.Buffer
	io.Copy(&buf, body)
	return buf.Bytes()
}

func NewJSONString(model interface{}) string {
	var buf = &bytes.Buffer{}
	json.NewEncoder(buf).Encode(model)
	return buf.String()
}

func DumpRequest(request *http.Request) {
	d, _ := httputil.DumpRequest(request, true)
	fmt.Printf("Request: %s\n", d)
}

func DumpResponse(response *http.Response) {
	d, _ := httputil.DumpResponse(response, true)
	fmt.Printf("Response: %s\n", d)
}

func UnixMiliSeconds() int64 {
	return time.Now().UnixNano() / 1000000
}

func SpritMagicToken(t string) []string {
	return strings.Split(t, ":")
}

func JsonBodyByScanLine(s string, start, end int) string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	i := 0
	var b string
	for scanner.Scan() {
		i++
		if i >= start && i <= end {
			b += scanner.Text()
		}
	}
	return b
}
