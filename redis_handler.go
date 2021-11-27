package delayed_job

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fd/go-shellwords/shellwords"
	"github.com/gomodule/redigo/redis"
)

type redisHandler struct {
	address  string
	password string
	client   *redis_gateway
	commands [][]string
}

func toStrings(v interface{}) ([]string, error) {
	switch s := v.(type) {
	case string:
		cmd, e := shellwords.Split(s)
		if nil != e {
			return nil, e
		}
		return cmd, nil
	case []string:
		if 0 == len(s) {
			return nil, errors.New("command is empty array")
		}
		return s, nil
	case []interface{}:
		if 0 == len(s) {
			return nil, errors.New("command is empty array")
		}
		cmd := make([]string, 0, len(s))
		for _, h := range s {
			if nil == h {
				break
			}
			cmd = append(cmd, fmt.Sprint(h))
		}
		return cmd, nil
	case map[string]interface{}:
		cmd := stringWithDefault(s, "command", "")
		if 0 == len(s) {
			return nil, errors.New("command is required")
		}

		return newRedisArguments(cmd,
			stringWithDefault(s, "arg0", ""),
			stringWithDefault(s, "arg1", ""),
			stringWithDefault(s, "arg2", ""),
			stringWithDefault(s, "arg3", ""),
			stringWithDefault(s, "arg4", ""),
			stringWithDefault(s, "arg5", ""),
			stringWithDefault(s, "arg6", "")), nil

	default:
		return nil, fmt.Errorf("command is unsupported type - %T", v)
	}
}

func newRedisArguments(arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7 string) []string {
	if "" == arg0 {
		return []string{}
	} else if "" == arg1 {
		return []string{arg0}
	} else if "" == arg2 {
		return []string{arg0, arg1}
	} else if "" == arg3 {
		return []string{arg0, arg1, arg2}
	} else if "" == arg4 {
		return []string{arg0, arg1, arg2, arg3}
	} else if "" == arg5 {
		return []string{arg0, arg1, arg2, arg3, arg4}
	} else if "" == arg6 {
		return []string{arg0, arg1, arg2, arg3, arg4, arg5}
	} else if "" == arg7 {
		return []string{arg0, arg1, arg2, arg3, arg4, arg5, arg6}
	} else {
		return []string{arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7}
	}
}

func replacePlaceHolders(cmd []string, arguments interface{}) ([]string, error) {

	holder := newValueHolder(arguments)

	for i := 0; i < len(cmd); i++ {
		if 1 >= len(cmd[i]) {
			continue
		}

		if "$$" == cmd[i] {
			bs, e := json.Marshal(arguments)
			if nil != e {
				return nil, errors.New("replace '$$' failed, " + e.Error())
			}
			cmd[i] = string(bs)
			continue
		}

		if '$' != cmd[i][0] {
			continue
		}

		o, e := holder.simpleValue(cmd[i][1:])
		if nil != e {
			return nil, errors.New("replace '" + cmd[i] + "' failed, " + e.Error())
		}
		cmd[i] = fmt.Sprint(o)
	}
	return cmd, nil
}

func newRedisHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == ctx {
		return nil, errors.New("ctx is nil")
	}
	if nil == params {
		return nil, errors.New("params is nil")
	}

	var address, password string
	var client *redis_gateway
	o, ok := ctx["redis"]
	if ok {
		client, ok = o.(*redis_gateway)
		if !ok {
			return nil, fmt.Errorf("'redis' in the ctx is not a *Redis - %T", o)
		}
		if nil == client {
			return nil, errors.New("'redis' in the ctx is nil")
		}
	} else {
		address = stringWithDefault(params, "address", "")
		password = stringWithDefault(params, "password", "")

		if address == "" {
			return nil, errors.New("'redis' in the ctx is required")
		}
	}

	args := params["arguments"]

	var array []string
	var e error
	o, ok = params["command"]
	if !ok {
		goto commands_label
	}

	array, e = toStrings(params)
	if nil != e {
		return nil, e
	}

	array, e = replacePlaceHolders(array, args)
	if nil != e {
		return nil, e
	}

	return &redisHandler{client: client, address: address, password: password, commands: [][]string{array}}, nil

commands_label:
	v, ok := params["commands"]
	if !ok {
		return nil, errors.New("'command' or 'commands' is required")
	}

	switch ss := v.(type) {
	case []string:
		if 0 == len(ss) {
			return nil, errors.New("commands is empty array")
		}
		commands := make([][]string, 0, len(ss))
		for _, s := range ss {
			cmd, e := shellwords.Split(s)
			if nil != e {
				return nil, e
			}

			cmd, e = replacePlaceHolders(cmd, args)
			if nil != e {
				return nil, e
			}

			commands = append(commands, cmd)
		}
		return &redisHandler{client: client, address: address, password: password, commands: commands}, nil
	case []interface{}:
		if 0 == len(ss) {
			return nil, errors.New("commands is empty array")
		}
		commands := make([][]string, 0, len(ss))
		for _, h := range ss {
			cmd, e := toStrings(h)
			if nil != e {
				return nil, e
			}

			cmd, e = replacePlaceHolders(cmd, args)
			if nil != e {
				return nil, e
			}

			commands = append(commands, cmd)
		}
		return &redisHandler{client: client, address: address, password: password, commands: commands}, nil
	default:
		return nil, fmt.Errorf("command is unsupported type - %T", v)
	}
}

func (self *redisHandler) Perform() error {
	if self.client == nil {
		dialOpts := []redis.DialOption{
			redis.DialWriteTimeout(1 * time.Second),
			redis.DialReadTimeout(1 * time.Second),
		}
		if self.password != "" {
			dialOpts = append(dialOpts, redis.DialPassword(self.password))
		}
		c, err := redis.Dial("tcp", self.address, dialOpts...)
		if err != nil {
			return fmt.Errorf("[redis] connect to '%s' failed, %v", self.address, err)
		}
		defer c.Close()

		for _, command := range self.commands {
			var args = make([]interface{}, len(command)-1)
			for idx := range command[1:] {
				args[idx] = command[idx+1]
			}

			c.Send(command[0], args...)
		}
		return nil
	}
	return self.client.Call(self.commands)
}

func init() {
	Handlers["redis"] = newRedisHandler
	Handlers["redis_command"] = newRedisHandler
}
