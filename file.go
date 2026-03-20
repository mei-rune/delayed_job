package delayed_job

import (
	"os"
	"errors"
	"io/ioutil"
	"path/filepath"
)

type writefileHandler struct {
	workDirectory string
	filename      string
	content       string
}

func newWriteFileHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == ctx {
		return nil, errors.New("ctx is nil")
	}
	if nil == params {
		return nil, errors.New("params is nil")
	}

	workDirectory := stringWithDefault(params, "work_directory", *default_directory)
	filename := stringWithDefault(params, "filename", "")
	content := stringWithDefault(params, "content", "")

	if args, ok := params["arguments"]; ok {
		args = preprocessArgs(args)

		if props, ok := args.(map[string]interface{}); ok {
			if _, ok := props["self"]; !ok {
				props["self"] = params
				defer delete(props, "self")
			}
		}
		var e error
		content, e = genText(content, args)
		if nil != e {
			return nil, e
		}

		filename, e = genText(filename, args)
		if nil != e {
			return nil, e
		}
	}
	return &writefileHandler{
		workDirectory: workDirectory,
		filename:      filename,
		content:       content,
	}, nil
}

func (h *writefileHandler) Perform() error {
	filename := filepath.Join(h.workDirectory, h.filename)
	if err := os.MkdirAll(filename, 0777); err != nil && os.IsExist(err) {
		return err
	}
	return ioutil.WriteFile(filename, []byte(h.content), 0666)
}

func init() {
	Handlers["file"] = newWriteFileHandler
	Handlers["file_command"] = newWriteFileHandler
}
