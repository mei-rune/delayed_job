package delayed_job

import (
	"errors"
	"strings"
	"sync"

	"gopkg.in/chanxuehong/wechat.v1/corp"
	"gopkg.in/chanxuehong/wechat.v1/corp/message/send"
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
	corp_server_url string
	corp_id     string
	corp_secret string
	msg         send.Text
}

func newWeixinHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}
	corp_server_url := stringWithDefault(params, "corp_server_url", "")
	corp_id := stringWithDefault(params, "corp_id", "")
	corp_secret := stringWithDefault(params, "corp_secret", "")
	target_type := stringWithDefault(params, "target_type", "")

	var msg send.Text
	msg.MsgType = send.MsgTypeText
	msg.AgentId = int64(intWithDefault(params, "agent_id", -1))
	if -1 == msg.AgentId {
		return nil, errors.New("agent_id is missing")
	}
	msg.Text.Content = stringWithDefault(params, "content", "")
	if "" == msg.Text.Content {
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

		msg.Text.Content, e = genText(msg.Text.Content, args)
		if nil != e {
			return nil, e
		}
	}

	switch strings.ToLower(target_type) {
	case "department", "departments", "departmentList", "departmentlist", "party":
		targets := stringOrArrayWithDefault(params, []string{"targets", "departmentList"}, "")
		if "" == targets {
			return nil, errors.New("department targets is empty")
		}

		msg.ToParty = strings.Replace(targets, ",", "|", -1)
	case "tag", "tags", "tagList", "taglist":
		targets := stringOrArrayWithDefault(params, []string{"targets", "tagList"}, "")
		if "" == targets {
			return nil, errors.New("tag targets is empty")
		}
		msg.ToTag = strings.Replace(targets, ",", "|", -1)
	default:
		targets := stringOrArrayWithDefault(params, []string{"targets", "userList"}, "")
		if "" == targets {
			return nil, errors.New(target_type + " targets is empty")
		}
		msg.ToUser = strings.Replace(targets, ",", "|", -1)
	}

	return &weixinHandler{
		corp_server_url: corp_server_url,
		corp_id: corp_id,
		corp_secret: corp_secret,
		msg:         msg}, nil
}

func (self *weixinHandler) Perform() error {
	if self.corp_server_url != "" {
		old := corp.QyApiURL
		corp.QyApiURL = self.corp_server_url

		defer func() {
			corp.QyApiURL = old
		}()
	}

	ul := GetWeixinClient(self.corp_id, Decrypt(self.corp_secret))
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
	Handlers["weixin_command"] = newWeixinHandler
}
