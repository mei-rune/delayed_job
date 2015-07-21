package main

import (
	"flag"
	"fmt"

	"github.com/runner-mei/delayed_job"
)

var (
	listenAddress = flag.String("listen", ":37078", "the address of http")
	run_mode      = flag.String("mode", "all", "init_db, console, backend, all")
)

func main() {
	flag.Parse()
	if nil != flag.Args() && 0 != len(flag.Args()) {
		flag.Usage()
		return
	}

	e := delayed_job.Main(*listenAddress, *run_mode)
	if nil != e {
		fmt.Println(e)
		return
	}
}
