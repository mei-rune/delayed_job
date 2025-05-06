package delayed_job

import (
	"errors"
	"time"

	"github.com/CodyGuo/dingtalk"
	"github.com/CodyGuo/dingtalk/pkg/robot"
)

func newDingHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}
	webhook := stringWithDefault(params, "webhook", "")
	if webhook == "" {
		webhook = stringWithDefault(params, "web_hook", "")
	}
	secret := stringWithDefault(params, "secret", "")
	content := stringWithDefault(params, "content", "")
	if content == "" {
		return nil, errors.New("content is missing")
	}

	var e error
	if args, ok := params["arguments"]; ok {
		args = preprocessArgs(args)
		if props, ok := args.(map[string]interface{}); ok {
			if _, ok := props["self"]; !ok {
				props["self"] = params
				defer delete(props, "self")
			}
		}

		content, e = genText(content, args)
		if nil != e {
			return nil, e
		}
	}

	targets := stringsWithDefault(params, "targets", ",", nil)
	if len(targets) == 0 {
		targets = stringsWithDefault(params, "userList", ",", nil)
	}

	return &dingHandler{
		webhook: webhook,
		secret:  secret,
		content: content,
		targets: targets,
	}, nil
}

type dingHandler struct {
	webhook string
	secret  string
	content string
	targets []string
}

func (self *dingHandler) Perform() error {
	if IsDevEnv {
		return ErrDevEnv
	}

	client := dingtalk.New(self.webhook,
		dingtalk.WithSecret(self.secret),
		dingtalk.WithTimeout(30*time.Second))
	// defer client.Close()

	var opts = []robot.SendOption{}
	if len(self.targets) > 0 {
		opts = append(opts, robot.SendWithAtMobiles(self.targets))
	}
	err := client.RobotSendText(self.content, opts...)
	if err != nil {
		if e, ok := err.(*dingtalk.Error); ok {
			return e.Unwrap()
		}
	}
	return err
}

func init() {
	Handlers["ding"] = newDingHandler
	Handlers["ding_action"] = newDingHandler
	Handlers["ding_command"] = newDingHandler
}
