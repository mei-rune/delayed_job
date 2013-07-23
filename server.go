package delayed_job

import (
	"flag"
)

func Main() error {
	flag.Parse()
	if nil != flag.Args() && 0 != len(flag.Args()) {
		flag.Usage()
		return nil
	}

	w, e := newWorker(map[string]interface{}{})
	if nil != e {
		return e
	}
	w.RunForever()
	return nil
}
