package gphoto

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	QueryNumberEnableImage           = 137530650
	QueryNumberGetAlbum              = 72930366
	QueryNumberCreateAlbum           = 79956622
	QueryNumberAddPhotoToAlbum       = 79956622
	QueryNumberAddPhotoToSharedAlbum = 99484733
	QueryNumberRemovePhotoFromAlbum  = 85381832
)

type Photo struct {
	ID string
}

type Album struct {
	ID   string
	Name string
}

type Albums []*Album

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

func (r EnableImageResponse) getEnabledImageId() string {
	innerArray := r[0].([]interface{})
	innerObject := innerArray[1].(map[string]interface{})
	secondInnerArray := innerObject[fmt.Sprintf("%v", QueryNumberEnableImage)].([]interface{})
	thirdInnerArray := secondInnerArray[0].([]interface{})
	fourthInnerArray := thirdInnerArray[0].([]interface{})
	fifthInnerObject := fourthInnerArray[1].([]interface{})
	return fifthInnerObject[0].(string)
}

func (eir EnableImageResponse) getEnabledImageURL() (string, error) {
	var inner3Array, inner6Array []interface{}
	if len(eir) > 0 {
		if inner1Array, ok := eir[0].([]interface{}); ok && len(inner1Array) >= 2 {
			if inner2Map, ok := inner1Array[1].(map[string]interface{}); ok {
				inner3Array = inner2Map[strconv.Itoa(QueryNumberEnableImage)].([]interface{})
			}
		}
	}
	if len(inner3Array) > 0 {
		if inner4Array, ok := inner3Array[0].([]interface{}); ok && len(inner4Array) > 0 {
			if inner5Array, ok := inner4Array[0].([]interface{}); ok && len(inner5Array) >= 2 {
				inner6Array = inner5Array[1].([]interface{})
			}
		}
	}
	if len(inner6Array) >= 2 {
		inner7Array := inner6Array[1].([]interface{})
		if enabledImageURL, ok := inner7Array[0].(string); ok {
			return enabledImageURL, nil
		}
	}
	return "", fmt.Errorf("no enabledImageURL")
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

func (albums Albums) Get(name string) *Album {
	for _, album := range albums {
		if name == album.Name {
			return album
		}
	}

	return nil
}
