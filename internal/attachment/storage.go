package attachment

import (
	"context"
	"io"
)

type Storage interface {
	Put(context.Context, string, string, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}
