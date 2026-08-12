package delayed_job

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/mitchellh/go-ps"
)

var pidFile *string

func init() {
	if "windows" == runtime.GOOS {
		pidFile = flag.String("job_pid_file", "delayed_job.pid", "File containing process PID")
	} else {
		pidFile = flag.String("job_pid_file", "/var/run/delayed_job.pid", "File containing process PID")
	}
}

func isPidInitialize() bool {
	ret := false
	flag.Visit(func(f *flag.Flag) {
		if "job_pid_file" == f.Name {
			ret = true
		}
	})
	return ret
}

func EnsureDefaultPidFile(filename string) {
	if !isPidInitialize() {
		flag.Set("job_pid_file", filename)
	}
}

func ensureProcessFile(nm string) {
	if "windows" == runtime.GOOS {
		EnsureDefaultPidFile(nm+".pid")
	} else {
		EnsureDefaultPidFile("/var/run/tpt/"+nm+".pid")
	}
}

func createPidFile(pidFile, image string) error {
	if pidString, err := ioutil.ReadFile(pidFile); err == nil {
		pid, err := strconv.Atoi(string(pidString))
		if err == nil {
			if pr, e := ps.FindProcess(pid); nil != e || (nil != pr &&
				strings.Contains(strings.ToLower(pr.Executable()), strings.ToLower(image))) {
				return fmt.Errorf("pid file found, ensure "+image+" is not running or delete %s", pidFile)
			}
		}
	}

	file, err := os.Create(pidFile)
	if err != nil {
		if e := os.MkdirAll(filepath.Dir(pidFile), 0666); e != nil {
			log.Println("[warn] mkdir '"+filepath.Dir(pidFile)+"' fail:", e)
		}
		file, err = os.Create(pidFile)
		if err != nil {
			return err
		}
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%d", os.Getpid())
	return err
}

func removePidFile(pidFile string) {
	if err := os.Remove(pidFile); err != nil {
		fmt.Printf("Error removing %s: %s\r\n", pidFile, err)
	}
}
