package delayed_job

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
)

var pidFile *string

func init() {
	if "windows" == runtime.GOOS {
		pidFile = flag.String("p", "delayed_job.pid", "File containing process PID")
	} else {
		pidFile = flag.String("p", "/var/run/delayed_job.pid", "File containing process PID")
	}
}

func createPidFile(pidFile string) error {
	if pidString, err := ioutil.ReadFile(pidFile); err == nil {
		pid, err := strconv.Atoi(string(pidString))
		if err == nil {
			if _, err = os.FindProcess(pid); nil == err {
				return fmt.Errorf("pid file found, ensure "+pidFile+" is not running or delete %s", pidFile)
			}
		}
	}

	file, err := os.Create(pidFile)
	if err != nil {
		return err
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
