package repository

import "errors"

// ErrNotFound is returned (wrapped) when a row the caller asked to change does
// not exist. Test for it with errors.Is.
var ErrNotFound = errors.New("not found")
