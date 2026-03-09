package storage

import "errors"

// ErrNotFound is returned when a query finds no matching rows.
var ErrNotFound = errors.New("not found")
