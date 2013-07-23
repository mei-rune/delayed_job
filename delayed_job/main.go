package main

import (
	"fmt"
	"github.com/runner-mei/delayed_job"
)

func main() {
	e := delayed_job.Main()
	if nil != e {
		fmt.Println(e)
		return
	}
}
