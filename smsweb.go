package delayed_job

import (
	"errors"
	"flag"
	"net/http"
	"strconv"

	"github.com/runner-mei/goutils/as"
)

var (
	smsWebBatchSupport    string
	smsWebMethod          string
	smsWebURL             string
	smsWebUsername        string
	smsWebPassword        string
	smsWebContentType     string
	smsWebBody            string
	smsWebHeaders         string
	smsWebResponseCode    string
	smsWebResponseContent string
)

func init() {
	flag.StringVar(&smsWebBatchSupport, "sms.web.batch_support", "", "")
	flag.StringVar(&smsWebMethod, "sms.web.method", "", "")
	flag.StringVar(&smsWebURL, "sms.web.url", "", "")
	flag.StringVar(&smsWebUsername, "sms.web.username", "", "")
	flag.StringVar(&smsWebPassword, "sms.web.password", "", "")
	flag.StringVar(&smsWebContentType, "sms.web.content_type", "", "")
	flag.StringVar(&smsWebBody, "sms.web.body", "", "")
	flag.StringVar(&smsWebHeaders, "sms.web.headers", "", "")
	flag.StringVar(&smsWebResponseCode, "sms.web.response_code", "", "")
	flag.StringVar(&smsWebResponseContent, "sms.web.response_content", "", "")
}

func BatchSendByWebSvc(args interface{}, phones []string, content string) error {
	if len(phones) == 0 {
		return nil
	}

	msg, e := genText(content, args)
	if nil != e {
		return e
	}

	params := map[string]interface{}{
		"args":    args,
		"phones":  phones,
		"phone":   phones[0],
		"content": msg,
	}

	urlStr, e := genText(smsWebURL, params)
	if nil != e {
		return errors.New("failed to merge 'url' with params, " + e.Error())
	}

	headerText, e := genText(smsWebHeaders, params)
	if nil != e {
		return errors.New("failed to merge 'headers' with params, " + e.Error())
	}
	headers := toKeyValues(headerText, nil)

	body, e := genText(smsWebBody, params)
	if nil != e {
		return errors.New("failed to merge 'body' with params, " + e.Error())
	}

	responseContent, e := genText(smsWebResponseContent, params)
	if nil != e {
		return errors.New("failed to merge 'response_content' with params, " + e.Error())
	}

	var responseCode = http.StatusOK
	if smsWebResponseCode != "" && smsWebResponseCode != "0" {
		i, e := strconv.Atoi(smsWebResponseCode)
		if nil != e {
			return errors.New("failed to merge 'response_code' with params, " + e.Error())
		}
		responseCode = i
	}

	handler := webHandler{
		method:          smsWebMethod,
		urlStr:          urlStr,
		user:            smsWebUsername,
		password:        smsWebPassword,
		contentType:     smsWebContentType,
		body:            body,
		headers:         headers,
		responseCode:    responseCode,
		responseContent: responseContent,
		args:            params,
		phoneNumbers:    phones,
		supportBatch:    as.BoolWithDefault(smsWebBatchSupport, false),
		isWebSMS:        true,
	}
	return handler.Perform()
}

func SendByWebSvc(args interface{}, phone, content string) error {
	return BatchSendByWebSvc(args, []string{phone}, content)
}
