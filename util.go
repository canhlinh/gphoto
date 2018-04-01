package gphoto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
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
	fmt.Printf("%s\n", d)
}

func DumpResponse(response *http.Response) {
	d, _ := httputil.DumpResponse(response, true)
	fmt.Printf("%s\n", d)
}
