package delayed_job

import (
	"os"
)

func processExistsByPid(pid int) bool {
	pids, e := enumProcesses()
	if nil != e {
		os.Stderr.WriteString("[warn] enum processes failed, " + e.Error() + "\r\n")
		return processExists(pid)
	}
	if _, ok := pids[pid]; ok {
		return true
	}
	return false
}

func processExists(pid int) bool {
	p, e := os.FindProcess(pid)
	if nil != e {
		if os.IsPermission(e) {
			return true
		}
		return false
	}
	p.Release()
	return true
}
