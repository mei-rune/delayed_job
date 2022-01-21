package delayed_job

import (
	"errors"
	"sync"

	"github.com/CatchZeng/dingtalk"
)

var dingLock sync.Mutex
var dingClients = map[string]*DingClient{}

type DingClient struct {
	client *dingtalk.Client
	mu     sync.Mutex
}

func GetDingClient(accessToken, secret string) *DingClient {
	dingLock.Lock()
	defer dingLock.Unlock()

	cl, ok := dingClients[accessToken+"-"+secret]
	if ok {
		return cl
	}

	cl = &DingClient{}
	cl.client = dingtalk.NewClient(accessToken, secret)

	dingClients[accessToken+"-"+secret] = cl
	return cl
}

type dingHandler struct {
	accessToken string
	secret      string
	msg         *dingtalk.TextMessage
}

func newDingHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}
	accessToken := stringWithDefault(params, "access_token", "")
	secret := stringWithDefault(params, "secret", "")

	msg := dingtalk.NewTextMessage()

	content := stringWithDefault(params, "content", "")
	if "" == content {
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

	msg.SetContent(content)

	targets := stringsWithDefault(params, "targets", ",", nil)
	if len(targets) == 0 {
		targets = stringsWithDefault(params, "userList", ",", nil)
	}
	if len(targets) == 0 {
		return nil, errors.New("targets is empty")
	}
	msg.SetAt(targets, false)

	return &dingHandler{
		accessToken: accessToken,
		secret:      secret,
		msg:         msg,
	}, nil
}

func (self *dingHandler) Perform() error {
	client := GetDingClient(self.accessToken, Decrypt(self.secret))
	client.mu.Lock()
	defer client.mu.Unlock()
	_, err := client.client.Send(self.msg)
	return err
}

func init() {
	Handlers["ding"] = newDingHandler
	Handlers["ding_action"] = newDingHandler
	Handlers["ding_command"] = newDingHandler
}
