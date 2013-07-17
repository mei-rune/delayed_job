package delayed_job

import (
	"errors"
)

type Handler interface {
	Perform() error
}

func newHandler(options map[string]interface{}) (Handler, error) {
	return nil, errors.New("not implemented")
}
