package gphoto

import (
	"io"
	"net/http"
	"os"
)

type ProgressHandler func(current int64, total int64)

type UploadResult struct {
	Err  error
	Resp *http.Response
}

type CopyBufferResult struct {
	Err     error
	Written int64
}

type Uploader struct {
	hClient *http.Client
}

func NewUploader(c *http.Client) *Uploader {
	return &Uploader{c}
}

func (u *Uploader) Do(url string, file io.Reader, fileSize int64, progressHanlder ProgressHandler) (*http.Response, error) {
	out, in, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	c1 := u.do(url, out, fileSize)
	c2 := copyBuffer(in, file, fileSize, progressHanlder)

	//Todo: Might leaked go routine here. Need check again

	for {
		select {
		case r1 := <-c1:
			return r1.Resp, r1.Err
		case r2 := <-c2:
			if r2.Err != nil {
				return nil, r2.Err
			}
		}
	}

}

func (u *Uploader) do(url string, file io.Reader, fileSize int64) chan *UploadResult {
	c := make(chan *UploadResult)

	go func() {

		result := &UploadResult{}
		defer func() {
			c <- result
		}()

		req, err := http.NewRequest(http.MethodPost, url, file)
		if err != nil {
			result.Err = err
			return
		}
		req.ContentLength = fileSize
		req.Header.Add("content-type", "application/octet-stream")
		req.Header.Add("user-agent", ChromeUserAgent)

		res, err := u.hClient.Do(req)
		if err != nil {
			result.Err = err
			return
		}

		result.Resp = res

	}()
	return c
}

func copyBuffer(dst io.Writer, src io.Reader, total int64, progressHanlder ProgressHandler) chan *CopyBufferResult {

	c := make(chan *CopyBufferResult)

	go func() {

		result := &CopyBufferResult{}
		defer func() {
			c <- result
		}()

		size := 32 * 1024
		buf := make([]byte, size)

		for {
			nr, er := src.Read(buf)
			if nr > 0 {
				nw, ew := dst.Write(buf[0:nr])
				if nw > 0 {
					result.Written += int64(nw)
					if progressHanlder != nil {
						progressHanlder(result.Written, total)
					}
				}
				if ew != nil {
					result.Err = ew
					break
				}
				if nr != nw {
					result.Err = io.ErrShortWrite
					break
				}
			}
			if er != nil {
				if er != io.EOF {
					result.Err = er
				}
				break
			}
		}

	}()

	return c
}
