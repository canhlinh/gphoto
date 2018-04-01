package gphoto

import "errors"

var (
	//ErrorUnknow For unexpected error
	ErrorUnknow = errors.New("Unknow Error")

	// ErrorAlbumNotCreatedYet In case no album was created just return it
	ErrorAlbumNotCreatedYet = errors.New("There is no album was created")
)
