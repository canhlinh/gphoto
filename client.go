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
	GoogleCommandDataURL = "https://photos.google.com/_/PhotosUi/data/batchexecute?f.sid=0&hl=en&soc-app=165&soc-platform=1&soc-device=1&_reqid=0&rt=c"

	// GoogleLoginSite the url to login
	GoogleLoginSite = "https://accounts.google.com/ServiceLogin"
	// DefaultAlbum a required album need to do a magic thing
	DefaultAlbum = "DefaultAlbum"
)

var (
	HomePageURL, _ = url.Parse(GooglePhotoURL)
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

	if err := c.parseAtToken(); err != nil {
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
	if err := c.parseAtToken(); err != nil {
		return nil, err
	}

	if filename == "" {
		filename = fileInfo.Name()
	}

	// Start create a new upload session
	uploadURL, err := c.createUploadURL(filename, fileInfo.Size())
	if err != nil {
		return nil, err
	}

	// start upload file
	uploadToken, err := c.upload(uploadURL, file, fileInfo.Size(), progressHandler)
	if err != nil {
		return nil, err
	}

	photoID, photoURL, err := c.enableUploadedFile(uploadToken, filename, fileInfo.ModTime().Unix()*1000)
	if err != nil {
		return nil, err
	}

	if album == "" {
		album = DefaultAlbum
	}

	photo, err := c.moveToAlbum(album, photoID)
	if err != nil {
		return nil, err
	}

	photo.Name = filename
	photo.URL = photoURL
	return photo, nil
}

//parseAtToken get the at token ( a magic token ) then set it as the magicToken
func (c *Client) parseAtToken() error {
	log.Info("Request to get the magic token")

	res, err := c.hClient.Get(GooglePhotoURL)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 {
		return errors.New(res.Status)
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
		return errors.New("Failed to get the magic token")
	}

	c.magicToken = magicToken.Token
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
	log.Info("Request to enable the uploaded photo")

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
	log.Info("Request to get albums")

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
	log.Info("Request to create new album %v with photo's id %s \n", albumName, photoID)

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

func (client *Client) GetSharedAlbumKey(albumID string) string {
	res, _ := client.hClient.Get(fmt.Sprintf("https://photos.google.com/u/0/album/%s", albumID))
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

func (c *Client) moveToAlbum(name string, photoID string) (*Photo, error) {
	log.Info("Request to move the upload file to the Default Album")

	albums, err := c.GetAlbums()
	if err != nil {
		return nil, err
	}

	album := albums.Get(name)
	photo := Photo{ID: photoID}

	if album == nil {
		album, err = c.CreateAlbum(name, photoID)
		if err != nil {
			return nil, err
		}
	} else if err := c.AddPhotoToAlbum(album.ID, photoID); err != nil {
		return nil, err
	}

	photo.AlbumID = album.ID
	return &photo, nil
}

func (c *Client) getPhotoURL(photoID string) (string, error) {
	return "", nil
}
