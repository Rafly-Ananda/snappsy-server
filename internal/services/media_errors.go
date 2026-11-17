package services

import "errors"

var (
	ErrUploadedObjectNotFound = errors.New("uploaded object was not found")
	ErrUnsupportedMediaType   = errors.New("unsupported media type")
	ErrInvalidObjectKey       = errors.New("object key does not belong to the event")
)
