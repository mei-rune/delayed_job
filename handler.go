package delayed_job

import (
	"errors"
)

type Handler interface {
	Perform() error
}
type Updater interface {
	UpdatePayloadObject(options map[string]interface{})
}

type MakeHandler func(ctx, options map[string]interface{}) (Handler, error)

var Handlers = map[string]MakeHandler{}

func newHandler(ctx, options map[string]interface{}) (Handler, error) {
	t := stringWithDefault(options, "type", "")
	if 0 == len(t) {
		return nil, errors.New("'type' is required.")
	}

	makeHandler := Handlers[t]
	if nil == makeHandler {
		return nil, errors.New("'" + t + "' is unsupported handler")
	}

	return makeHandler(ctx, options)
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

func (self testHandler) UpdatePayloadObject(options map[string]interface{}) {
	options["UpdatePayloadObject"] = "UpdatePayloadObject"
}

func newTest(ctx, options map[string]interface{}) (Handler, error) {
	return testHandler(options), nil
}

func init() {
	Handlers["test"] = newTest
}
