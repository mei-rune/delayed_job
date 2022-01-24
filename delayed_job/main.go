package main

import (
	"flag"
	"fmt"

	"github.com/runner-mei/delayed_job"
)

var (
	db_url        = flag.String("db_url", "host=127.0.0.1 dbname=delayed_test user=delayedtest password=123456 sslmode=disable", "the db url")
	db_drv        = flag.String("db_drv", "postgres", "the db driver")
	listenAddress = flag.String("listen", ":37078", "the address of http")
	run_mode      = flag.String("mode", "all", "init_db, console, backend, all")
)

func main() {
	flag.Parse()
	if nil != flag.Args() && 0 != len(flag.Args()) {
		flag.Usage()
		return
	}

	e := delayed_job.Main(db_drv, db_url, *listenAddress, *run_mode)
	if nil != e {
		fmt.Println(e)
		return
	}
}
