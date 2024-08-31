package pkg

import "errors"

var (
	NotFoundErr = errors.New("not found")
	BusyErr     = errors.New("busy now")
)
