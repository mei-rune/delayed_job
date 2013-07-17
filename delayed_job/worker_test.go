package delayed_job

import (
	"testing"
	"time"
)

func TestWorker(t *testing.T) {
	backend, e := newBackend(*db_drv, *db_url)
	if nil != e {
		t.Error(e)
		return
	}
	defer backend.Close()

	w := newWorker(backend, map[string]interface{}{})
	time.Sleep(1 * time.Second)
	w.Close()

	_, _, e = w.work_off(10)
	if nil != e {
		t.Error(e)
	}
}
