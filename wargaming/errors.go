package wargaming

import "errors"

var (
	ErrPOWTooHard         = errors.New("pow too hard")
	ErrCSRFNotFound       = errors.New("csrf not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrMergeRequired      = errors.New("merge required")
	ErrInvalidCaptcha     = errors.New("invalid captcha")
)
