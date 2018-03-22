package gphoto

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/publicsuffix"
)

const (
	// ChromeUserAgent user-agent of chrome browser
	ChromeUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36"

	// GooglePhotoURL the google photo homepage
	GooglePhotoURL = "https://photos.google.com"

	// GooglePhotoRequestUploadURL url to request create a new upload session
	GooglePhotoRequestUploadURL = "https://photos.google.com/_/upload/uploadmedia/rupio/interactive?authuser=0"

	// GooglePhotoCommandURL url to execute a specific command
	GooglePhotoCommandURL = "https://photos.google.com/_/PhotosUi/mutate"

	// EnablePhotoKey a magic key
	EnablePhotoKey = 137530650
)

// Client present a upload client
type Client struct {
	hClient *http.Client
}

func NewClient(cookies ...*http.Cookie) *Client {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	hClient := &http.Client{
		Jar: jar,
	}

	c := &Client{
		hClient: hClient,
	}

	return c.SetCookies(cookies...)
}

//SetCookies attach google's cookies to the upload client
func (c *Client) SetCookies(cookies ...*http.Cookie) *Client {

	for _, cookie := range cookies {
		cookie.Path = "/"
		switch cookie.Name {
		case "OTZ":
			cookie.Domain = "photos.google.com"
		case "PAIDCONTENT":
			cookie.Path = "/insights/consumersurveys"
			cookie.Domain = ".www.google.com"
		default:
			cookie.Domain = ".google.com"
		}
	}

	cookieURL, _ := url.Parse(GooglePhotoURL)
	c.hClient.Jar.SetCookies(cookieURL, cookies)
	return c
}

func (c *Client) ExportCookies(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		log.Fatal(err)
	}

	cookieURL, _ := url.Parse(GooglePhotoURL)
	json.NewDecoder(file).Decode(c.hClient.Jar.Cookies(cookieURL))
}

// SetHTTPClient specific the http client to the upload client.
func (c *Client) SetHTTPClient(hClient *http.Client) *Client {
	c.hClient = hClient
	return c
}

// Upload uploads the file to the google photo.
// We will recive an url that people can access to the uploaded file directly.
func (c *Client) Upload(filePath string) (string, string, error) {
	log.Println("Start upload file ", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	fileInfo, _ := file.Stat()

	// A magic token need to be genarate firstly.
	atToken, err := c.getAtToken()
	if err != nil {
		return "", "", err
	}

	// Start create a new upload session
	uploadURL, err := c.createUploadURL(fileInfo.Name(), fileInfo.Size())
	if err != nil {
		return "", "", err
	}
	log.Println("Got new upload url " + uploadURL)

	// start upload file
	uploadToken, err := c.upload(uploadURL, file, fileInfo.Size())
	if err != nil {
		return "", "", err
	}

	return c.enableUploadedFile(uploadToken, atToken, fileInfo.Name(), fileInfo.ModTime().Unix()*1000)
}

//getAtToken get the at token ( a magic token )
func (c *Client) getAtToken() (string, error) {
	res, err := c.hClient.Get(GooglePhotoURL)
	if err != nil {
		return "", err
	}

	doc, _ := goquery.NewDocumentFromReader(res.Body)

	var magicToken MagicToken
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		if strings.HasPrefix(s.Text(), "window.WIZ_global_data") {
			script := s.Text()
			scriptObject := script[strings.Index(script, "{"):]
			scriptObject = scriptObject[:strings.LastIndex(scriptObject, "}")+1]

			json.Unmarshal([]byte(scriptObject), &magicToken)
			return
		}
	})

	if magicToken.Token == "" {
		return "", errors.New("Failed to get the magic token")
	}

	return magicToken.Token, nil
}

//createUploadURL create an new upload url
func (c *Client) createUploadURL(fileName string, fileSize int64) (string, error) {

	body := NewJSONBody(NewUploadSessionRequest(fileName, fileSize))

	req, _ := http.NewRequest(http.MethodPost, GooglePhotoRequestUploadURL, body)
	req.Header.Add("content-type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Add("user-agent", ChromeUserAgent)

	resp, err := c.hClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return "", fmt.Errorf("Failed to create a new upload's id, got error %s", StringFromBody(resp.Body))
	}

	result := NewSessionUploadFromJson(StringFromBody(resp.Body))
	if len(result.SessionStatus.ExternalFieldTransfers) <= 0 {
		return "", errors.New("An array of the request URL response is empty")
	}

	return result.SessionStatus.ExternalFieldTransfers[0].PutInfo.URL, nil
}

// upload uploads file to server then you will get a upload token
func (c *Client) upload(uploadURL string, file io.ReadCloser, fileSize int64) (string, error) {

	req, _ := http.NewRequest(http.MethodPost, uploadURL, file)
	req.Header.Add("content-type", "application/octet-stream")
	req.Header.Add("user-agent", ChromeUserAgent)

	resp, err := c.hClient.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode > 299 {
		return "", fmt.Errorf("Failed to upload file, got error %s", StringFromBody(resp.Body))
	}

	stringBody := StringFromBody(resp.Body)
	uploadToken := NewSessionUploadFromJson(stringBody).SessionStatus.AdditionalInfo.GoogleRupioAdditionalInfo.CompletionInfo.CustomerSpecificInfo.UploadToken
	if uploadToken == "" {
		log.Println(stringBody)
		return "", fmt.Errorf("Failed to get upload token")
	}
	return uploadToken, nil
}

func (c *Client) enableUploadedFile(uploadBase64Token, atToken, fileName string, fileModAt int64) (string, string, error) {

	jsonReq := EnableImageRequest{
		"af.maf",
		[]FirstItemEnableImageRequest{
			[]InnerItemFirstItemEnableImageRequest{
				"af.add",
				EnablePhotoKey,
				SecondInnerArray{
					MapOfItemsToEnable{
						fmt.Sprintf("%v", EnablePhotoKey): ItemToEnable{
							ItemToEnableArray{
								[]InnerItemToEnableArray{
									uploadBase64Token,
									fileName,
									fileModAt,
								},
							},
						},
					},
				},
			},
		},
	}

	form := url.Values{}
	form.Add("f.req", StringFromBody(NewJSONBody(jsonReq)))
	form.Add("at", atToken)

	req, _ := http.NewRequest(http.MethodPost, GooglePhotoCommandURL, strings.NewReader(form.Encode()))
	req.Header.Add("user-agent", ChromeUserAgent)
	req.Header.Add("content-type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Add("authority", "photos.google.com")
	req.Header.Add("origin", GooglePhotoURL)
	req.Header.Add("referer", GooglePhotoURL)
	req.Header.Add("x-chrome-uma-enabled", "1")
	req.Header.Add("x-same-domain", "1")
	resp, err := c.hClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return "", "", fmt.Errorf("Failed to enable the uploaded file, got error %s", StringFromBody(resp.Body))
	}

	bytesResponse, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var enableImage EnableImageResponse
	if err := json.Unmarshal(bytesResponse[6:], &enableImage); err != nil {
		return "", "", err
	}

	photoURL, err := enableImage.getEnabledImageURL()
	if err != nil {
		return "", "", err
	}

	return enableImage.getEnabledImageId(), photoURL, nil
}
