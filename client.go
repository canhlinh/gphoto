package gphoto

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	// GooglePhotoMutateQueryURL url to execute a specific command
	GooglePhotoMutateQueryURL = "https://photos.google.com/_/PhotosUi/mutate"

	// GooglePhotoDataQueryURL url do something
	GooglePhotoDataQueryURL = "https://photos.google.com/_/PhotosUi/data"

	// DefaultAlbum a required album need to do a magic thing
	DefaultAlbum = "DefaultAlbum"
)

// Client present a upload client
type Client struct {
	hClient    *http.Client
	magicToken string
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

	c.magicToken = atToken

	// Start create a new upload session
	uploadURL, err := c.createUploadURL(fileInfo.Name(), fileInfo.Size())
	if err != nil {
		return "", "", err
	}

	// start upload file
	uploadToken, err := c.upload(uploadURL, file, fileInfo.Size())
	if err != nil {
		return "", "", err
	}

	photoID, photoURL, err := c.enableUploadedFile(uploadToken, fileInfo.Name(), fileInfo.ModTime().Unix()*1000)
	if err != nil {
		return "", "", err
	}

	if err := c.moveToAlbum(DefaultAlbum, photoID); err != nil {
		return "", "", err
	}

	return photoID, photoURL, err

}

//getAtToken get the at token ( a magic token )
func (c *Client) getAtToken() (string, error) {
	log.Println("Request to get the magic token")

	res, err := c.hClient.Get(GooglePhotoURL)
	if err != nil {
		return "", err
	}

	if res.StatusCode > 299 {
		return "", errors.New(res.Status)
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
	log.Println("Request to create a new upload url")

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
		return "", fmt.Errorf("Failed to create a new upload's id, got error %s", BodyToString(resp.Body))
	}

	result := NewSessionUploadFromJson(BodyToString(resp.Body))
	if len(result.SessionStatus.ExternalFieldTransfers) <= 0 {
		return "", errors.New("An array of the request URL response is empty")
	}

	return result.SessionStatus.ExternalFieldTransfers[0].PutInfo.URL, nil
}

// upload uploads file to server then you will get a upload token
func (c *Client) upload(uploadURL string, file io.ReadCloser, fileSize int64) (string, error) {
	log.Println("Request to upload file data")

	req, _ := http.NewRequest(http.MethodPost, uploadURL, file)
	req.Header.Add("content-type", "application/octet-stream")
	req.Header.Add("user-agent", ChromeUserAgent)

	resp, err := c.hClient.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode > 299 {
		return "", fmt.Errorf("Failed to upload file, got error %s", BodyToString(resp.Body))
	}

	stringBody := BodyToString(resp.Body)
	uploadToken := NewSessionUploadFromJson(stringBody).SessionStatus.AdditionalInfo.GoogleRupioAdditionalInfo.CompletionInfo.CustomerSpecificInfo.UploadToken
	if uploadToken == "" {
		log.Println(stringBody)
		return "", fmt.Errorf("Failed to get upload token")
	}
	return uploadToken, nil
}

func (c *Client) enableUploadedFile(uploadBase64Token, fileName string, fileModAt int64) (string, string, error) {
	log.Println("Request to enable the uploaded photo")

	query := NewMutateQuery(QueryNumberEnableImage,
		[]interface{}{
			[]interface{}{
				[]interface{}{
					uploadBase64Token,
					fileName,
					fileModAt,
				},
			},
		},
	)

	body, err := c.DoQuery(GooglePhotoMutateQueryURL, query)
	if err != nil {
		return "", "", err
	}

	var enableImage EnableImageResponse
	if err := json.Unmarshal(BodyToBytes(body)[6:], &enableImage); err != nil {
		return "", "", err
	}

	photoURL, err := enableImage.getEnabledImageURL()
	if err != nil {
		return "", "", err
	}

	return enableImage.getEnabledImageId(), photoURL, nil
}

func (client *Client) DoQuery(endpoint string, query string) (io.ReadCloser, error) {

	form := url.Values{}
	form.Add("at", client.magicToken)
	form.Add("f.req", query)

	req, _ := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Add("User-Agent", ChromeUserAgent)

	res, err := client.hClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode > 299 {
		// DumpRequest(req)
		// DumpResponse(res)
		return nil, errors.New(res.Status)
	}

	return res.Body, nil
}

func (client *Client) GetAlbums() (Albums, error) {
	log.Println("Request to get albums")

	body, err := client.DoQuery(GooglePhotoDataQueryURL, NewDataQuery(QueryNumberGetAlbum, []interface{}{nil, nil, nil, nil, 1}))
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var r []interface{}
	if err := json.Unmarshal(BodyToBytes(body)[6:], &r); err != nil {
		return nil, err
	}

	var albums = Albums{}

	r = r[0].([]interface{})
	r = r[2].(map[string]interface{})[fmt.Sprintf("%v", QueryNumberGetAlbum)].([]interface{})

	if len(r) == 0 {
		return albums, nil
	}

	r = r[0].([]interface{})

	for _, al := range r {
		infos := al.([]interface{})
		mdetails := infos[12].(map[string]interface{})
		details := mdetails[fmt.Sprintf("%v", QueryNumberGetAlbum)].([]interface{})

		albums = append(albums, &Album{
			ID:   infos[0].(string),
			Name: details[1].(string),
		})
	}

	return albums, nil
}

func (c *Client) SearchOrCreteaAlbum(name string, photoID string) (*Album, error) {
	albums, err := c.GetAlbums()
	if err != nil {
		return nil, err
	}

	for _, album := range albums {
		if name == album.Name {
			return album, nil
		}
	}

	return c.CreateAlbum(name, photoID)
}

func (c *Client) CreateAlbum(albumName string, photoID string) (*Album, error) {
	log.Printf("Request to create new album %v with photo's id %s \n", albumName, photoID)

	query := NewMutateQuery(QueryNumberCreateAlbum, []interface{}{
		[]string{photoID},
		nil,
		albumName,
	})

	body, err := c.DoQuery(GooglePhotoMutateQueryURL, query)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var album Album
	var r interface{}
	if err := json.Unmarshal(BodyToBytes(body)[6:], &r); err != nil {
		return nil, err
	}

	r = (r.([]interface{})[0]).([]interface{})[1]
	r = r.(map[string]interface{})[fmt.Sprintf("%d", QueryNumberCreateAlbum)]
	album.ID = r.([]interface{})[0].(string)
	album.Name = albumName

	return &album, nil
}

func (client *Client) AddPhotoToAlbum(albumID, photoID string) error {
	log.Printf("Request to add photo %s to album %s", photoID, albumID)

	query := NewMutateQuery(QueryNumberAddPhotoToAlbum,
		[]interface{}{
			[]string{photoID},
			albumID,
		},
	)

	body, err := client.DoQuery(GooglePhotoMutateQueryURL, query)
	if err != nil {
		return err
	}
	body.Close()

	return nil
}

func (client *Client) AddPhotoToSharedAlbum(albumID, photoID string) error {
	log.Printf("Request to add photo %s to shared album %s", photoID, albumID)

	query := NewMutateQuery(QueryNumberAddPhotoToSharedAlbum,
		[]interface{}{
			[]string{albumID},
			[]interface{}{
				2,
				nil,
				[]interface{}{
					[][]string{[]string{photoID}},
				},
				nil,
				nil,
				[]interface{}{},
				[]interface{}{},
			},
		},
	)

	body, err := client.DoQuery(GooglePhotoMutateQueryURL, query)
	if err != nil {
		return err
	}
	body.Close()

	return nil
}

func (c *Client) RemoveFromAlbum(photoID string) error {
	log.Printf("Request to remove photo %s from the relevant album", photoID)

	query := NewMutateQuery(
		QueryNumberRemovePhotoFromAlbum,
		[]interface{}{
			[]string{photoID},
			[]interface{}{},
		},
	)

	body, err := c.DoQuery(GooglePhotoMutateQueryURL, query)
	if err != nil {
		return err
	}
	defer body.Close()
	return nil
}

func (c *Client) moveToAlbum(name string, photoID string) error {
	log.Println("Request to move the upload file to the Default Album")

	albums, err := c.GetAlbums()
	if err != nil {
		return err
	}

	album := albums.Get(name)
	if album == nil {
		if _, err := c.CreateAlbum(name, photoID); err != nil {
			return err
		}
		return nil
	}

	if err := c.AddPhotoToAlbum(album.ID, photoID); err != nil {
		return c.AddPhotoToSharedAlbum(album.ID, photoID)
	}

	return nil
}
