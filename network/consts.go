package network

import "errors"

var (
	errEmptyResponse   = errors.New("empty response")
	errInvalidResponse = errors.New("invalid result")
)
