package delayed_job

import (

	//"net/http"
	//_ "net/http/pprof"
	"flag"
	"runtime"
	"strings"
	"testing"
)

func TestExecHandlerParameterIsError(t *testing.T) {
	_, e := newExecHandler(nil, map[string]interface{}{})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "ctx is nil" != e.Error() {
		t.Error("excepted error is 'ctx is nil', but actual is", e)
	}

	_, e = newExecHandler(map[string]interface{}{}, nil)
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "params is nil" != e.Error() {
		t.Error("excepted error is 'params is nil', but actual is", e)
	}

	_, e = newExecHandler(map[string]interface{}{}, map[string]interface{}{})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'command' is required." != e.Error() {
		t.Error("excepted error is ['command' is required.], but actual is", e)
	}
}

func TestExecHandlerNoExe(t *testing.T) {
	handler, e := newExecHandler(map[string]interface{}{}, map[string]interface{}{"command": "aaa"})
	if nil != e {
		t.Error(e)
		return
	}
	e = handler.Perform()
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
		return
	}

	if !strings.Contains(e.Error(), "exec: \"aaa\": executable file not found") {
		t.Error("excepted error contains [exec: \"aaa\": executable file not found], but actual is", e)
	}
}

func TestExecHandlerNoWorkDirectory(t *testing.T) {
	handler, e := newExecHandler(map[string]interface{}{}, map[string]interface{}{"command": "go version", "work_directory": "aaaa"})
	if nil != e {
		t.Error(e)
		return
	}
	e = handler.Perform()
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
		return
	}

	if !strings.Contains(e.Error(), "chdir aaaa:") {
		t.Error("excepted error contains [chdir aaaa:], but actual is", e)
	}
}

func TestExecHandlerNoPrompt(t *testing.T) {
	handler, e := newExecHandler(map[string]interface{}{}, map[string]interface{}{"command": "go version"})
	if nil != e {
		t.Error(e)
		return
	}
	e = handler.Perform()
	if nil != e {
		t.Error(e)
		return
	}
}

func TestExecHandlerPromptNotFound(t *testing.T) {
	// go func() {
	// 	http.ListenAndServe(":7078", nil)
	// }()
	handler, e := newExecHandler(map[string]interface{}{}, map[string]interface{}{"command": "go version", "prompt": "aasddf"})
	if nil != e {
		t.Error(e)
		return
	}
	e = handler.Perform()
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
		return
	}
	if !strings.Contains(e.Error(), "************************* not found ************************") {
		t.Error("excepted error contains [not found], but actual is", e)
	}
}

func TestExecHandlerPromptFound(t *testing.T) {
	// go func() {
	// 	http.ListenAndServe(":7078", nil)
	// }()
	handler, e := newExecHandler(map[string]interface{}{}, map[string]interface{}{"command": "go version", "prompt": runtime.GOARCH})
	if nil != e {
		t.Error(e)
		return
	}
	e = handler.Perform()
	if nil != e {
		t.Error(e)
		return
	}
}

func TestExecHandlerArguments(t *testing.T) {
	// go func() {
	// 	http.ListenAndServe(":7078", nil)
	// }()
	var args map[string]interface{}
	if "windows" == runtime.GOOS {
		args = map[string]interface{}{"command": "cmd /c echo \"{{.a1}}\"", "prompt": "ccc", "arguments": map[string]interface{}{"a1": "ccc"}}
	} else {
		args = map[string]interface{}{"command": "echo \"{{.a1}}\"", "prompt": "ccc", "arguments": map[string]interface{}{"a1": "ccc"}}
	}

	handler, e := newExecHandler(map[string]interface{}{}, args)
	if nil != e {
		t.Error(e)
		return
	}
	e = handler.Perform()
	if nil != e {
		t.Error(e)
		return
	}
}

func TestExecHandlerWorkdirectory(t *testing.T) {
	// go func() {
	//  http.ListenAndServe(":7078", nil)
	// }()
	var args map[string]interface{}
	if "windows" == runtime.GOOS {
		args = map[string]interface{}{"command": "cmd /c echo %%cd%%", "prompt": "window", "work_directory": "c:\\windows\\"}
	} else {
		args = map[string]interface{}{"command": "pwd", "prompt": "usr", "work_directory": "/usr/"}
	}

	handler, e := newExecHandler(map[string]interface{}{}, args)
	if nil != e {
		t.Error(e)
		return
	}
	e = handler.Perform()
	if nil != e {
		t.Error(e)
		return
	}
}

var plink_work_directory = flag.String("plink_work_directory", "C:\\Program Files (x86)\\hengwei", "")
var plink_command = flag.String("plink_command", "", "")
var plink_prompt = flag.String("plink_prompt", "aaa", "")

func TestExecPlink(t *testing.T) {
	// go func() {
	//  http.ListenAndServe(":7078", nil)
	// }()
	if "windows" != runtime.GOOS || "" == *plink_command {
		return
	}
	args := map[string]interface{}{"command": *plink_command, "prompt": *plink_prompt, "work_directory": *plink_work_directory}

	handler, e := newExecHandler(map[string]interface{}{}, args)
	if nil != e {
		t.Error(e)
		return
	}
	e = handler.Perform()
	if nil != e {
		t.Error(e)
		return
	}
}
