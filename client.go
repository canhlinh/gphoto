package gphoto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/canhlinh/log4go"
	"github.com/sclevine/agouti"
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

	// GoogleCommandDataURL
	GoogleCommandDataURL = "https://photos.google.com/_/PhotosUi/data/batchexecute?f.sid=0&bl=boq_photosuiserver_20180711.03_p0&hl=en&soc-app=165&soc-platform=1&soc-device=1&_reqid=785335&rt=c"

	// GoogleLoginSite the url to login
	GoogleLoginSite = "https://accounts.google.com/ServiceLogin"
	// DefaultAlbum a required album need to do a magic thing
	DefaultAlbum = "DefaultAlbum"
)

var (
	HomePageURL, _ = url.Parse(GooglePhotoURL)
	regex1         = regexp.MustCompile(`"SNlM0e":"[a-zA-Z0-9_-]+:\d+"`)
	regex2         = regexp.MustCompile(`\n\[\["wrb.fr","mdpdU","(.*?)"\]\]\n`)
	regex3         = regexp.MustCompile(`\n\[\["wrb.fr","Z5xsfc",(.*?)\]\]\n`)
	regex4         = regexp.MustCompile(`\n\[\["wrb.fr","OXvT9d",(.*?)\]\]\n`)
)

// Client present a upload client
type Client struct {
	hClient    *http.Client
	magicToken string
	uploader   *Uploader
}

// NewClient init a Client by existing cookies.
func NewClient(cookies ...*http.Cookie) *Client {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	hClient := &http.Client{
		Jar: jar,
	}

	c := &Client{
		hClient:  hClient,
		uploader: NewUploader(hClient),
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

	c.hClient.Jar.SetCookies(HomePageURL, cookies)
	return c
}

func (c *Client) ExportCookies() string {
	var buf bytes.Buffer
	json.NewDecoder(&buf).Decode(c.hClient.Jar.Cookies(HomePageURL))
	return buf.String()
}

// SetHTTPClient specific the http client to the upload client.
func (c *Client) SetHTTPClient(hClient *http.Client) *Client {
	c.hClient = hClient
	return c
}

// Login login to google photo with your authentication info.
func (c *Client) Login(user, pass string) error {
	log.Info("Request to login")

	drive := agouti.ChromeDriver()
	if err := drive.Start(); err != nil {
		return err
	}

	defer drive.Stop()

	page, err := drive.NewPage(agouti.Browser("firefox"))
	if err != nil {
		return err
	}
	defer page.CloseWindow()

	if err := page.Navigate(GoogleLoginSite); err != nil {
		return err
	}

	page.Find("form").FirstByName("identifier").Fill(user)
	if err := page.FindByID("identifierNext").Click(); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	if err := page.Find("form").FindByName("password").Fill(pass); err != nil {
		return err
	}
	if err := page.FindByID("passwordNext").Click(); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	page.Navigate(GooglePhotoURL)
	cookies, _ := page.GetCookies()
	c.hClient.Jar.SetCookies(HomePageURL, cookies)

	if err := c.parseMagicToken(); err != nil {
		return errors.New("Login failure. Can not get the magic token")
	}

	log.Info("Login successful")
	return nil
}

// Upload uploads the file to the google photo.
// We will recive an url that people can access to the uploaded file directly.
func (c *Client) Upload(filePath string, filename string, album string, progressHandler ProgressHandler) (*Photo, error) {
	log.Info("Start upload file %s", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fileInfo, _ := file.Stat()

	// A magic token need to be genarate firstly.
	if err := c.parseMagicToken(); err != nil {
		return nil, err
	}

	if filename == "" {
		filename = fileInfo.Name()
	}

	// Start create a new upload session
	uploadURL, err := c.createUploadURL(filename, fileInfo.Size())
	if err != nil {
		log.Error("Failed to create upload url, got error %s", err.Error())
		return nil, err
	}

	// start upload file
	uploadToken, err := c.upload(uploadURL, file, fileInfo.Size(), progressHandler)
	if err != nil {
		log.Error("Failed to upload data, got error %s", err.Error())
		return nil, err
	}
	fmt.Println("uploadToken:", uploadToken)

	photoID, photoURL, err := c.enableUploadedFile(uploadToken, filename, fileInfo.ModTime().UnixNano()/1000000)
	if err != nil {
		log.Error("Failed to enable upload url, got error %s", err.Error())
		return nil, err
	}
	fmt.Println("photoID:", photoID)

	if album == "" {
		album = DefaultAlbum
	}

	photo, err := c.moveToAlbum(album, photoID)
	if err != nil {
		log.Error("Failed to move the photo to album, got error %s", err.Error())
		return nil, err
	}

	photo.Name = filename
	photo.URL = photoURL
	return photo, nil
}

//parseMagicToken get the at token ( a magic token ) then set it as the magicToken
func (c *Client) parseMagicToken() error {
	log.Info("Request to get the magic token")

	res, err := c.hClient.Get(GooglePhotoURL)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 {
		return errors.New(res.Status)
	}

	doc, _ := goquery.NewDocumentFromReader(res.Body)
	s := doc.Text()
	s = regex1.FindString(s)
	if len(s) == 0 {
		return errors.New("Failed to get the magic token")
	}
	s = strings.TrimLeft(s, `"SNlM0e":`)
	s = strings.Trim(s, `"`)
	c.magicToken = s
	return nil
}

//createUploadURL create an new upload url
func (c *Client) createUploadURL(fileName string, fileSize int64) (string, error) {
	log.Info("Request to create a new upload url")

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
func (c *Client) upload(uploadURL string, file io.ReadCloser, fileSize int64, progressHandler ProgressHandler) (string, error) {
	log.Info("Request to upload file data")

	resp, err := c.uploader.Do(uploadURL, file, fileSize, progressHandler)
	if err != nil {
		return "", err
	}

	if resp.StatusCode > 299 {
		return "", fmt.Errorf("Failed to upload file, got error %s", BodyToString(resp.Body))
	}

	stringBody := BodyToString(resp.Body)
	uploadToken := NewSessionUploadFromJson(stringBody).SessionStatus.AdditionalInfo.GoogleRupioAdditionalInfo.CompletionInfo.CustomerSpecificInfo.UploadToken
	if uploadToken == "" {
		log.Info(stringBody)
		return "", fmt.Errorf("Failed to get upload token")
	}
	return uploadToken, nil
}

func (c *Client) enableUploadedFile(uploadBase64Token, fileName string, fileModAt int64) (string, string, error) {
	log.Info("Request to enable the uploaded photo %d", fileModAt)
	query := fmt.Sprintf(`[[["mdpdU","[[[\"%s\",\"%s\",%d]]]",null,"generic"]]]`, uploadBase64Token, fileName, fileModAt)
	body, err := c.DoQuery(GoogleCommandDataURL, query)
	if err != nil {
		log.Error(err)
		return "", "", err
	}
	defer body.Close()

	s := BodyToString(body)
	s = regex2.FindString(s)
	log.Debug(s)
	var enableImage EnableImageResponse
	photoURL, err := enableImage.getEnabledImageURL(s)
	if err != nil {
		log.Error(err)
		return "", "", err
	}
	photoID, err := enableImage.getEnabledImageID(s)
	if err != nil {
		log.Error(err)
		return "", "", err
	}

	return photoID, photoURL, nil
}

// DoQuery executes http request
func (client *Client) DoQuery(endpoint string, query string) (io.ReadCloser, error) {
	if client.magicToken == "" {
		return nil, errors.New("Empty magic token")
	}

	form := url.Values{}
	form.Add("at", client.magicToken)
	form.Add("f.req", query)

	req, _ := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Add("User-Agent", ChromeUserAgent)
	req.Header.Add("referer", "https://photos.google.com/")
	res, err := client.hClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode > 299 {
		return nil, errors.New(res.Status)
	}

	return res.Body, nil
}

// GetAlbums gets all google photo albums
func (client *Client) GetAlbums() (Albums, error) {
	log.Info("Request to get albums")
	body, err := client.DoQuery(GoogleCommandDataURL, `[[["Z5xsfc","[null,null,null,null,1]",null,"3"]]]`)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	s := BodyToString(body)
	s = regex3.FindString(s)
	log.Debug(s)

	albumlResponse := NewAlbumlResponse(s)
	albums, err := albumlResponse.Albums()
	if err != nil {
		return nil, err
	}

	return albums, nil
}

// SearchOrCreteaAlbum creates an album if the album name doesn't exist
func (c *Client) SearchOrCreteaAlbum(name string) (*Album, error) {
	albums, err := c.GetAlbums()
	if err != nil {
		return nil, err
	}

	if len(albums) == 0 {
		return nil, errors.New("No album is found")
	}

	for _, album := range albums {
		if name == album.Name {
			return album, nil
		}
	}

	return c.CreateAlbum(name)
}

// CreateAlbum creates a new album
func (c *Client) CreateAlbum(albumName string) (*Album, error) {
	log.Info("Request to create new album %s", albumName)

	query := fmt.Sprintf(`[[["OXvT9d","[\"%s\",null,2,[]]",null,"generic"]]]`, albumName)
	endpoint := GoogleCommandDataURL + "&rpcids=OXvT9d"

	body, err := c.DoQuery(endpoint, query)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	s := BodyToString(body)
	s = regex4.FindString(s)

	var arr []interface{}
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil, err
	}

	arr = arr[0].([]interface{})
	id := arr[2].(string)
	var o interface{}
	if err := json.Unmarshal([]byte(s), &o); err != nil {
		return nil, err
	}
	arr = o.([]interface{})
	arr = arr[0].([]interface{})
	id = arr[0].(string)

	album := &Album{
		ID:   id,
		Name: albumName,
	}

	return album, nil
}

// GetSharedAlbumKey gets an album's share key
func (c *Client) GetSharedAlbumKey(albumID string) string {
	res, _ := c.hClient.Get(fmt.Sprintf("https://photos.google.com/u/0/album/%s", albumID))
	if res.StatusCode != 200 {
		return ""
	}

	doc, _ := goquery.NewDocumentFromReader(res.Body)

	var sharedKey string
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		v, _ := s.Attr("http-equiv")
		c, _ := s.Attr("content")
		if v == "refresh" && len(strings.Split(c, ";")) == 2 {
			redirectURL, _ := url.Parse(strings.Split(c, ";")[1])
			sharedKey = redirectURL.Query().Get("key")
		}
	})

	return sharedKey
}

// AddPhotoToAlbum adds a photo to an album
func (c *Client) AddPhotoToAlbum(albumID, photoID string) error {
	log.Info("Request to add photo %s to album %s", photoID, albumID)
	sharedAlbumKey := c.GetSharedAlbumKey(albumID)

	var query string

	if len(sharedAlbumKey) == 0 {
		query = fmt.Sprintf(`[[["E1Cajb","[[\"%s\"],\"%s\"]",null,"generic"]]]`, photoID, albumID)
	} else {
		query = fmt.Sprintf(`[[["C2V01c","[[\"%s\"],[2,null,[[[\"%s\"]]],null,null,[],[1],null,null,null,[]],\"%s\",[null,null,null,null,[null,[]]]]",null,"generic"]]]`, albumID, photoID, sharedAlbumKey)
	}

	body, err := c.DoQuery(GoogleCommandDataURL, query)
	if err != nil {
		return err
	}
	defer body.Close()

	return nil
}

// RemoveFromAlbum Remove a photo from an album
func (c *Client) RemoveFromAlbum(photoID string) error {
	log.Info("Request to remove photo %s from the relevant album", photoID)

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

// moveToAlbum move a photo to an album
func (c *Client) moveToAlbum(albumName string, photoID string) (*Photo, error) {
	log.Info("Request to move the upload file to the album %s", albumName)

	albums, err := c.GetAlbums()
	if err != nil {
		log.Error("Failed to get album %s", err.Error())
		return nil, err
	}

	album := albums.Get(albumName)
	photo := Photo{ID: photoID}

	if album == nil {
		album, err = c.CreateAlbum(albumName)
		if err != nil {
			log.Error("Failed to create new album %s", err.Error())
			return nil, err
		}
	}

	if err := c.AddPhotoToAlbum(album.ID, photoID); err != nil {
		log.Error("Failed to add photo to existing album %s", err.Error())
		return nil, err
	}

	log.Debug("Added phonto:%d to album:%s", photoID, album.ID)
	photo.AlbumID = album.ID
	return &photo, nil
}

func (c *Client) getPhotoURL(photoID string) (string, error) {
	return "", nil
}
