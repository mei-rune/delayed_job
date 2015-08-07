package delayed_job

import (
	"errors"
	"strings"
	"sync"

	"github.com/chanxuehong/wechat/corp"
	"github.com/chanxuehong/wechat/corp/message/send"
)

var weixin_lock sync.Mutex
var weixin_clients = map[string]*WeixinClient{}

type WeixinClient struct {
	client *send.Client
	mu     sync.Mutex
}

func GetWeixinClient(corp_id, corp_secret string) *WeixinClient {
	weixin_lock.Lock()
	defer weixin_lock.Unlock()

	cl, ok := weixin_clients[corp_id+"-"+corp_secret]
	if ok {
		return cl
	}

	cl = &WeixinClient{}
	var accessTokenServer = corp.NewDefaultAccessTokenServer(corp_id, corp_secret, nil)
	cl.client = send.NewClient(accessTokenServer, nil)

	weixin_clients[corp_id+"-"+corp_secret] = cl
	return cl
}

type weixinHandler struct {
	corp_id     string
	corp_secret string
	msg         send.Text
}

func newWeixinHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}
	corp_id := stringWithDefault(params, "corp_id", "")
	corp_secret := stringWithDefault(params, "corp_secret", "")
	target_type := stringWithDefault(params, "target_type", "")
	targets := stringWithDefault(params, "targets", "")
	if "" == targets {
		return nil, errors.New("targets is empty.")
	}

	var msg send.Text
	msg.MsgType = send.MsgTypeText
	msg.AgentId = int64(intWithDefault(params, "agent_id", -1))
	if -1 == msg.AgentId {
		return nil, errors.New("agent_id is missing.")
	}
	msg.Text.Content = stringWithDefault(params, "content", "")
	if "" == msg.Text.Content {
		return nil, errors.New("content is missing.")
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

		msg.Text.Content, e = genText(msg.Text.Content, args)
		if nil != e {
			return nil, e
		}
	}

	switch strings.ToLower(target_type) {
	case "department", "departments", "party":
		msg.ToParty = targets
	case "tag", "tags":
		msg.ToTag = targets
	default:
		msg.ToUser = targets
	}

	return &weixinHandler{corp_id: corp_id,
		corp_secret: corp_secret,
		msg:         msg}, nil
}

func (self *weixinHandler) Perform() error {
	ul := GetWeixinClient(self.corp_id, self.corp_secret)
	ul.mu.Lock()
	defer ul.mu.Unlock()

	if r, err := ul.client.SendText(&self.msg); nil != err {
		return err
	} else if "" != r.InvalidUser {
		return errors.New("invalid user - " + r.InvalidUser)
	} else if "" != r.InvalidParty {
		return errors.New("invalid party - " + r.InvalidParty)
	} else if "" != r.InvalidTag {
		return errors.New("invalid tag - " + r.InvalidUser)
	}
	return nil
}

func init() {
	Handlers["weixin"] = newWeixinHandler
	Handlers["weixin_action"] = newWeixinHandler
}
