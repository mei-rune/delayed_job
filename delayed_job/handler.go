package delayed_job

import (
	"errors"
)

type Handler interface {
	Perform() error
}

func newHandler(options map[string]interface{}) (Handler, error) {
	t := stringWithDefault(options, "type", "")
	if 0 == len(t) {
		return nil, errors.New("'type' is required.")
	}

	switch t {
	case "test":
		return newTest(options)
	}
	return nil, errors.New("'" + t + "' is unsupported handler")
}

var test_chan = make(chan map[string]interface{}, 100)

type testHandler map[string]interface{}

func (self testHandler) Perform() error {
	test_chan <- self
	e := stringWithDefault(self, "error", "")
	if 0 == len(e) {
		return nil
	}

	return errors.New(e)
}

func newTest(options map[string]interface{}) (Handler, error) {
	return testHandler(options), nil
}
