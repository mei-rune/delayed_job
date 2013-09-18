package delayed_job

import (
	"testing"
)

func TestPidfileExist(t *testing.T) {
	if e := createPidFile("./delayed_job.pid"); nil != e {
		t.Error(e)
	}
}
