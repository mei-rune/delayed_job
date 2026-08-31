package delayed_job

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-lark/lark"
)

func newFeishuHandler(ctx, params map[string]interface{}) (Handler, error) {
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

	return &feishuHandler{
		webhook: webhook,
		secret:  secret,
		content: content,
		targets: targets,
	}, nil
}

type feishuHandler struct {
	webhook string
	secret  string
	content string
	targets []string
}

func (self *feishuHandler) Perform() error {
	if IsDevEnv {
		return ErrDevEnv
	}

	bot := lark.NewNotificationBot(self.webhook)
	bot.SetClient(&http.Client{
		Timeout: 30 * time.Second,
	})

	content := self.content
	if len(self.targets) > 0 {
		var atParts []string
		for _, t := range self.targets {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if strings.HasPrefix(t, "ou_") || strings.HasPrefix(t, "on_") || strings.HasPrefix(t, "uu_") {
				atParts = append(atParts, fmt.Sprintf("<at user_id=\"%s\">%s</at>", t, t))
			} else if strings.HasPrefix(t, "oc_") {
				atParts = append(atParts, fmt.Sprintf("<at chat_id=\"%s\">%s</at>", t, t))
			} else {
				atParts = append(atParts, fmt.Sprintf("<at phone=\"%s\">%s</at>", t, t))
			}
		}
		if len(atParts) > 0 {
			content = content + "\n" + strings.Join(atParts, " ")
		}
	}

	msgBuf := lark.NewMsgBuffer(lark.MsgText).Text(content)
	if self.secret != "" {
		msgBuf = msgBuf.WithSign(self.secret, time.Now().Unix())
	}

	resp, err := bot.PostNotificationV2(msgBuf.Build())
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("feishu bot error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

func init() {
	Handlers["feishu"] = newFeishuHandler
	Handlers["feishu_action"] = newFeishuHandler
	Handlers["feishu_command"] = newFeishuHandler
	Handlers["lark"] = newFeishuHandler
	Handlers["lark_action"] = newFeishuHandler
	Handlers["lark_command"] = newFeishuHandler
}
