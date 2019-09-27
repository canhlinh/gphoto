package gphoto

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	log "github.com/canhlinh/log4go"
)

const (
	QueryNumberEnableImage           = 137530650
	QueryNumberGetAlbum              = 72930366
	QueryNumberCreateAlbum           = 79956622
	QueryNumberAddPhotoToAlbum       = 79956622
	QueryNumberAddPhotoToSharedAlbum = 99484733
	QueryNumberRemovePhotoFromAlbum  = 85381832
	QueryStringAddPhotoToAlbum       = "C2V01c"
)

type Photo struct {
	ID      string
	AlbumID string
	Name    string
	URL     string
}

type Album struct {
	ID   string
	Name string
}

type Albums []*Album

func (albums Albums) Get(name string) *Album {
	for _, album := range albums {
		if name == album.Name {
			return album
		}
	}

	return nil
}

type MagicToken struct {
	Token string `json:"SNlM0e"`
}

type ExternalField struct {
	Field interface{} `json:"external"`
}

type ExternalFieldNewUpload struct {
	Name     string   `json:"name"`
	FileName string   `json:"filename"`
	Put      struct{} `json:"put"`
	Size     int64    `json:"size"`
}

type ExternalFieldTransfer struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	PutInfo struct {
		URL string `json:"url"`
	} `json:"putInfo"`
}

type InlinedFieldObject struct {
	Name        string `json:"name"`
	Content     string `json:"contentType"`
	ContentType string `json:"contentType"`
}

type InlinedField struct {
	Inlined InlinedFieldObject `json:"inlined"`
}

type SessionRequest struct {
	ProtocolVersion      string               `json:"protocolVersion"`
	CreateSessionRequest CreateSessionRequest `json:"createSessionRequest"`
}

type CreateSessionRequest struct {
	Fields []interface{} `json:"fields"`
}

func NewUploadSessionRequest(fileName string, fileSize int64) *SessionRequest {

	sessionRequest := &SessionRequest{
		ProtocolVersion: "0.8",
		CreateSessionRequest: CreateSessionRequest{
			Fields: []interface{}{
				ExternalField{
					Field: ExternalFieldNewUpload{
						Name:     "file",
						FileName: fileName,
						Size:     fileSize,
					},
				},
			},
		},
	}

	return sessionRequest
}

type GoogleRupioAdditionalInfo struct {
	CompletionInfo struct {
		CustomerSpecificInfo struct {
			UploadToken string `json:"upload_token_base64"`
		} `json:"customerSpecificInfo"`
	} `json:"completionInfo"`
}

type SessionStatus struct {
	State                  string                   `json:"state"`
	ExternalFieldTransfers []*ExternalFieldTransfer `json:"externalFieldTransfers"`
	UploadID               string                   `json:"upload_id"`
	DropZoneLabel          string                   `json:"drop_zone_label"`
	AdditionalInfo         struct {
		GoogleRupioAdditionalInfo GoogleRupioAdditionalInfo `json:"uploader_service.GoogleRupioAdditionalInfo"`
	} `json:"additionalInfo"`
}

type SessionUpload struct {
	SessionStatus SessionStatus `json:"sessionStatus"`
}

func NewSessionUploadFromJson(body string) *SessionUpload {
	var sessionUpload SessionUpload
	json.NewDecoder(strings.NewReader(body)).Decode(&sessionUpload)

	return &sessionUpload
}

type EnableImageResponse []interface{}

// getInfoArray un-safe function
func (r EnableImageResponse) getInfoArray(s string) []interface{} {
	json.Unmarshal([]byte(s), &r)
	b := r[0].([]interface{})
	obj := b[2].(string)
	json.Unmarshal([]byte(obj), &b)

	b = b[0].([]interface{})
	b = b[0].([]interface{})
	b = b[1].([]interface{})
	return b
}

// getEnabledImageID un-safe function
func (r EnableImageResponse) getEnabledImageID(body string) (s string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	infoArr := r.getInfoArray(body)
	return infoArr[0].(string), nil
}

// getEnabledImageURL un-safe function
func (r EnableImageResponse) getEnabledImageURL(body string) (s string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	infoArr := r.getInfoArray(body)
	infoURL := infoArr[1].([]interface{})
	return infoURL[0].(string), err
}

func NewDataQuery(queryNumber int, query interface{}) string {
	d, _ := json.Marshal(
		[]interface{}{
			[]interface{}{
				[]interface{}{
					queryNumber,
					[]interface{}{
						map[string]interface{}{
							fmt.Sprintf("%v", queryNumber): query,
						},
					},
					nil,
					nil,
					0,
				},
			},
		},
	)
	return fmt.Sprintf("%s", d)
}

func NewMutateQuery(queryNumber int, query interface{}) string {
	d, _ := json.Marshal([]interface{}{
		"af.maf",
		[]interface{}{
			[]interface{}{
				"af.add",
				queryNumber,
				[]interface{}{
					map[string]interface{}{
						fmt.Sprintf("%v", queryNumber): query,
					},
				},
			},
		}})
	return fmt.Sprintf("%s", d)
}

type AlbumlResponse struct {
	s string
}

func NewAlbumlResponse(s string) *AlbumlResponse {
	return &AlbumlResponse{s}
}

func (al *AlbumlResponse) Albums() (albums []*Album, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
			log.Error(err)
		}
	}()

	mainArray := al.getMainArray()

	log.Warn(NewJSONString(mainArray))

	for _, arr := range mainArray {
		albumID, albumName := al.getAlbumInfo(arr.([]interface{}))
		albums = append(albums, &Album{
			ID:   albumID,
			Name: albumName,
		})
	}
	return albums, nil
}

func (al *AlbumlResponse) getMainArray() []interface{} {
	var b []interface{}
	json.Unmarshal([]byte(al.s), &b)
	b = b[0].([]interface{})
	obj := b[2].(string)
	json.Unmarshal([]byte(obj), &b)

	b = b[0].([]interface{})
	return b
}

func (al *AlbumlResponse) getAlbumInfo(b []interface{}) (string, string) {

	for _, c := range b {
		if reflect.ValueOf(c).Kind() == reflect.Map {
			innerarray := c.(map[string]interface{})["72930366"].([]interface{})
			return b[0].(string), innerarray[1].(string)
		}
	}
	return "", ""
}
