package gphoto

import (
	"bytes"
	"encoding/json"
	"io"
)

//NewJSONBody create a new json request body from an interface
func NewJSONBody(model interface{}) io.Reader {
	var buf = &bytes.Buffer{}
	json.NewEncoder(buf).Encode(model)
	return buf
}

func StringFromBody(body io.Reader) string {
	var buf bytes.Buffer
	io.Copy(&buf, body)
	return buf.String()
}

func NewJSONString(model interface{}) string {
	var buf = &bytes.Buffer{}
	json.NewEncoder(buf).Encode(model)
	return buf.String()
}
