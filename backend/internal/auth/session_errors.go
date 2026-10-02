package auth

import "errors"

var ErrSessionInvalid = errors.New("invalid or expired human session")
var ErrSessionUnavailable = errors.New("human session authority unavailable")
