package delayed_job

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fd/go-shellwords/shellwords"
	"github.com/kardianos/osext"
)

var default_directory = flag.String("exec.directory", ".", "the work directory for execute")

type execHandler struct {
	work_directory string
	prompt         string
	command        string
	arguments      []string
	environments   []string
}

func newExecHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == ctx {
		return nil, errors.New("ctx is nil")
	}
	if nil == params {
		return nil, errors.New("params is nil")
	}

	work_directory := stringWithDefault(params, "work_directory", *default_directory)
	prompt := stringWithDefault(params, "prompt", "")
	command := stringWithDefault(params, "command", "")
	environments := stringsWithDefault(params, "environments", ";", nil)
	if 0 == len(command) {
		return nil, errors.New("'command' is required.")
	}

	if args, ok := params["arguments"]; ok {
		args = preprocessArgs(args)

		if props, ok := args.(map[string]interface{}); ok {
			if _, ok := props["self"]; !ok {
				props["self"] = params
				defer delete(props, "self")
			}
		}

		var e error
		command, e = genText(command, args)
		if nil != e {
			return nil, e
		}
		prompt, e = genText(prompt, args)
		if nil != e {
			return nil, e
		}
		for idx, s := range environments {
			s, e = genText(s, args)
			if nil != e {
				return nil, e
			}
			environments[idx] = s
		}
	}

	arguments, e := shellwords.Split(command)
	if nil != e {
		return nil, errors.New("split shell command failed, " + e.Error())
	}

	return &execHandler{work_directory: work_directory,
		prompt:       prompt,
		command:      arguments[0],
		arguments:    arguments[1:],
		environments: environments}, nil
}

func newExecHandler2(ctx, params map[string]interface{}) (Handler, error) {
	if nil == ctx {
		return nil, errors.New("ctx is nil")
	}
	if nil == params {
		return nil, errors.New("params is nil")
	}

	work_directory := stringWithDefault(params, "work_directory", *default_directory)
	prompt := stringWithDefault(params, "prompt", "")
	environments := stringsWithDefault(params, "environments", ";", nil)
	command := stringWithDefault(params, "command", "")
	if 0 == len(command) {
		return nil, errors.New("'command' is required.")
	}

	var arguments []string
	if args, ok := params["arguments"]; ok {
		var e error
		arguments, e = toStrings(args)
		if nil != e {
			return nil, e
		}
	}

	if args, ok := params["options"]; ok {
		args = preprocessArgs(args)

		if props, ok := args.(map[string]interface{}); ok {
			if _, ok := props["self"]; !ok {
				props["self"] = params
				defer delete(props, "self")
			}
		}

		var e error
		command, e = genText(command, args)
		if nil != e {
			return nil, e
		}
		prompt, e = genText(prompt, args)
		if nil != e {
			return nil, e
		}

		for idx, s := range arguments {
			s, e = genText(s, args)
			if nil != e {
				return nil, e
			}
			arguments[idx] = s
		}

		for idx, s := range environments {
			s, e = genText(s, args)
			if nil != e {
				return nil, e
			}
			environments[idx] = s
		}
	}

	return &execHandler{work_directory: work_directory,
		prompt:       prompt,
		command:      command,
		arguments:    arguments,
		environments: environments}, nil
}

var ExecutableFolder string

func init() {
	executableFolder, e := osext.ExecutableFolder()
	if nil != e {
		fmt.Println("[warn]", e)
		return
	}

	ExecutableFolder = executableFolder
}
func lookPath(executableFolder string, alias ...string) (string, bool) {
	var names []string
	for _, aliasName := range alias {
		if runtime.GOOS == "windows" {
			names = append(names, aliasName, aliasName+".bat", aliasName+".com", aliasName+".exe")
		} else {
			names = append(names, aliasName, aliasName+".sh")
		}
	}

	for _, nm := range names {
		files := []string{nm,
			filepath.Join("bin", nm),
			filepath.Join("tools", nm),
			filepath.Join("runtime_env", nm),
			filepath.Join("..", nm),
			filepath.Join("..", "bin", nm),
			filepath.Join("..", "tools", nm),
			filepath.Join("..", "runtime_env", nm),
			filepath.Join(executableFolder, nm),
			filepath.Join(executableFolder, "bin", nm),
			filepath.Join(executableFolder, "tools", nm),
			filepath.Join(executableFolder, "runtime_env", nm),
			filepath.Join(executableFolder, "..", nm),
			filepath.Join(executableFolder, "..", "bin", nm),
			filepath.Join(executableFolder, "..", "tools", nm),
			filepath.Join(executableFolder, "..", "runtime_env", nm)}
		for _, file := range files {
			// fmt.Println("====", file)
			file = abs(file)
			if st, e := os.Stat(file); nil == e && nil != st && !st.IsDir() {
				//fmt.Println("1=====", file, e)
				return file, true
			}
		}
	}

	for _, nm := range names {
		_, err := exec.LookPath(nm)
		if nil == err {
			return nm, true
		}
	}
	return "", false
}

func (self *execHandler) Perform() error {
	if "tpt" == self.command || "tpt.exe" == self.command {
		if a, ok := lookPath(ExecutableFolder, "tpt"); ok {
			self.command = a
		}
	} else {
		if a, ok := lookPath(ExecutableFolder, self.command); ok {
			self.command = a
		}
	}

	fmt.Println(self.command, self.arguments)
	cmd := exec.Command(self.command, self.arguments...)
	cmd.Dir = self.work_directory

	var environments []string
	if len(self.environments) > 0 {
		os_env := os.Environ()
		environments = make([]string, 0, len(self.arguments)+len(os_env))
		environments = append(environments, os_env...)
		environments = append(environments, self.environments...)
		cmd.Env = environments
	}

	if 0 == len(self.prompt) {
		var buffer bytes.Buffer
		cmd.Stderr = &buffer
		cmd.Stdout = cmd.Stderr

		err := cmd.Start()
		if err != nil {
			return err
		}

		c := make(chan error, 1)
		go func() {
			c <- cmd.Wait()
		}()

		timer := time.NewTimer(10 * time.Minute)

		select {
		case <-timer.C:
			cmd.Process.Kill()
			return ErrTimeout
		case err := <-c:
			timer.Stop()
			if err != nil {
				buffer.WriteString("\r\n ************************* exit *************************\r\n")
				buffer.WriteString(err.Error())
				return errors.New(buffer.String())
			}
			return nil
		}

		return nil
	}

	pr, pw := io.Pipe()
	//if err != nil {
	//	return errors.New("create pipe failed, " + err.Error())
	//}
	defer func() {
		pr.Close()
		pw.Close()
	}()

	if strings.Contains(strings.ToLower(cmd.Path), "plink") {
		cmd.Stdin = strings.NewReader("y\ny\ny\ny\ny\ny\ny\ny\n")
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	var scan_error error
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()

		buffer := bytes.NewBuffer(make([]byte, 0, 10240))
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), self.prompt) {
				return
			}
			buffer.Write(scanner.Bytes())

			if buffer.Len() > 10*1024*1024 {
				buffer.WriteString("\r\n ************************* read too large *************************\r\n")
				goto end
			}
		}
		buffer.WriteString("\r\n ************************* prompt `" + self.prompt + "` not found *************************\r\n")
	end:
		scan_error = errors.New(buffer.String())
	}()

	timer := time.AfterFunc(10*time.Minute, func() {
		defer recover()
		cmd.Process.Kill()
	})
	err := cmd.Run()

	timer.Stop()
	pw.Close()
	pr.Close()
	wait.Wait()
	if nil != err {
		if nil != scan_error {
			return errors.New("start cmd failed, " + err.Error() + "\r\n" + scan_error.Error())
		}
		return errors.New("start cmd failed, " + err.Error())
	}

	return scan_error
}

func init() {
	Handlers["exec"] = newExecHandler
	Handlers["exec_command"] = newExecHandler
	Handlers["exec2"] = newExecHandler2
	Handlers["exec2_command"] = newExecHandler2
}
