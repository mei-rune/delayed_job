package delayed_job

import (
	"flag"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	var flagSet flag.FlagSet
	a := flagSet.Int("a", -1, "for test")
	b := flagSet.Bool("b", true, "for test")
	c := flagSet.String("c", "-1", "for test")
	da := flagSet.Int("d.a", -1, "for test")
	dd := flagSet.Bool("d.d", false, "for test")
	dc := flagSet.String("d.c", "-1", "for test")
	e := LoadConfig("config_test.txt", &flagSet, false)
	if nil != e {
		t.Error(e)
		return
	}

	if *a != 1 {
		t.Error("a != 1")
	}
	if !*b {
		t.Error("b != true")
	}
	if "abc" != *c {
		t.Error("c != \"abc\"")
	}
	if *da != 1323 {
		t.Error("d.a != 1323")
	}
	if !*dd {
		t.Error("db != true")
	}
	if "67" != *dc {
		t.Error("dc != \"67\", actual is %s", *dc)
	}
}

func TestLoadConfigWithNotDefined(t *testing.T) {
	var flagSet flag.FlagSet
	a := flagSet.Int("a", -1, "for test")
	b := flagSet.Bool("b", true, "for test")
	c := flagSet.String("c", "-1", "for test")
	da := flagSet.Int("d.a", -1, "for test")
	dc := flagSet.String("d.c", "-1", "for test")
	e := LoadConfig("config_test.txt", &flagSet, false)
	if nil != e {
		t.Error(e)
		return
	}

	if *a != 1 {
		t.Error("a != 1")
	}
	if !*b {
		t.Error("b != true")
	}
	if "abc" != *c {
		t.Error("c != \"abc\"")
	}
	if *da != 1323 {
		t.Error("d.a != 1323")
	}

	if "67" != *dc {
		t.Error("dc != \"67\", actual is %s", *dc)
	}
}

func TestLoadConfigWithNotOverride(t *testing.T) {
	var flagSet flag.FlagSet
	a := flagSet.Int("a", -1, "for test")
	b := flagSet.Bool("b", true, "for test")
	c := flagSet.String("c", "-1", "for test")
	da := flagSet.Int("d.a", -1, "for test")
	dc := flagSet.String("d.c", "-1", "for test")
	*a = 2
	*b = false
	*c = "set"
	*da = 2
	*dc = "set2"

	e := LoadConfig("config_test.txt", &flagSet, false)
	if nil != e {
		t.Error(e)
		return
	}

	if *a != 2 {
		t.Error("a != 2")
	}
	if *b {
		t.Error("b != true")
	}
	if "set" != *c {
		t.Error("c != \"set\"")
	}
	if *da != 2 {
		t.Error("d.a != 2")
	}

	if "set2" != *dc {
		t.Error("dc != \"set2\", actual is %s", *dc)
	}
}

func TestLoadConfigWithOverride(t *testing.T) {
	var flagSet flag.FlagSet
	a := flagSet.Int("a", -1, "for test")
	b := flagSet.Bool("b", true, "for test")
	c := flagSet.String("c", "-1", "for test")
	da := flagSet.Int("d.a", -1, "for test")
	dc := flagSet.String("d.c", "-1", "for test")
	*a = 2
	*b = false
	*c = "set"
	*da = 2
	*dc = "set2"

	e := LoadConfig("config_test.txt", &flagSet, true)
	if nil != e {
		t.Error(e)
		return
	}

	if *a != 1 {
		t.Error("a != 1")
	}
	if !*b {
		t.Error("b != true")
	}
	if "abc" != *c {
		t.Error("c != \"abc\"")
	}
	if *da != 1323 {
		t.Error("d.a != 1323")
	}

	if "67" != *dc {
		t.Error("dc != \"67\", actual is %s", *dc)
	}
}
