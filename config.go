package delayed_job

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

func LoadConfig(nm string, flagSet *flag.FlagSet, isOverride bool) error {
	f, e := os.Open(nm)
	if nil != e {
		return fmt.Errorf("load config '%s' failed, %v", nm, e)
	}

	var res map[string]interface{}
	e = json.NewDecoder(f).Decode(&res)
	if nil != e {
		return fmt.Errorf("load config '%s' failed, %v", nm, e)
	}

	e = assignFlagSet("", res, flagSet, isOverride)
	if nil != e {
		return fmt.Errorf("load config '%s' failed, %v", nm, e)
	}
	return nil
}

func assignFlagSet(prefix string, res map[string]interface{}, flagSet *flag.FlagSet, isOverride bool) error {
	for k, v := range res {
		switch value := v.(type) {
		case map[string]interface{}:
			e := assignFlagSet(combineName(prefix, k), value, flagSet, isOverride)
			if nil != e {
				return e
			}
			continue
		case []interface{}:
		case string:
		case float64:
		case bool:
		case nil:
			continue
		default:
			return fmt.Errorf("unsupported type for %s - %T", combineName(prefix, k), v)
		}

		g := flagSet.Lookup(combineName(prefix, k))
		if nil == g {
			log.Printf("flag '%s' is not defined.\n", combineName(prefix, k))
			continue
		}
		if !isOverride && g.Value.String() != g.DefValue {
			continue
		}

		err := g.Value.Set(fmt.Sprint(v))
		if nil != err {
			return err
		}
	}
	return nil
}

func combineName(prefix, nm string) string {
	if "" == prefix {
		return nm
	}
	return prefix + "." + nm
}
