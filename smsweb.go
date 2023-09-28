package delayed_job

import (
	"errors"
	"flag"
	"net/http"
	"strconv"
  "strings"

	"tech.hengwei.com.cn/go/goutils/as"
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

  webURL := readStringWith(args, "sms.web.url", smsWebURL)
	urlStr, e := genText(webURL, params)
	if nil != e {
		return errors.New("failed to merge 'url' with params, " + e.Error())
	}

  txt := readStringWith(args, "sms.web.headers", smsWebHeaders)
	headerText, e := genText(txt, params)
	if nil != e {
		return errors.New("failed to merge 'headers' with params, " + e.Error())
	}
	headers := toKeyValues(headerText, nil)

  txt = readStringWith(args, "sms.web.body", smsWebBody)
	body, e := genText(txt, params)
	if nil != e {
		return errors.New("failed to merge 'body' with params, " + e.Error())
	}

  txt = readStringWith(args, "sms.web.response_content", smsWebResponseContent)
	responseContent, e := genText(txt, params)
	if nil != e {
		return errors.New("failed to merge 'response_content' with params, " + e.Error())
	}

  txt = readStringWith(args, "sms.web.response_code", smsWebResponseCode)
	var responseCode = http.StatusOK
	if txt != "" && txt != "0" {
		i, e := strconv.Atoi(txt)
		if nil != e {
			return errors.New("failed to merge 'response_code' with params, " + e.Error())
		}
		responseCode = i
	}

  batchSupport := readStringWith(args, "sms.web.batch_support", smsWebBatchSupport)
  method := readStringWith(args, "sms.web.method", smsWebMethod)

	handler := webHandler{
		method:          strings.ToUpper(method),
		urlStr:          urlStr,
		user:            readStringWith(args, "sms.web.username", smsWebUsername),
		password:        readStringWith(args, "sms.web.password", smsWebPassword),
		contentType:     readStringWith(args, "sms.web.content_type", smsWebContentType),
		body:            body,
		headers:         headers,
		responseCode:    responseCode,
		responseContent: responseContent,
		args:            params,
		phoneNumbers:    phones,
		supportBatch:    as.BoolWithDefault(batchSupport, false),
		isWebSMS:        true,
	}
	return handler.Perform()
}

func SendByWebSvc(args interface{}, phone, content string) error {
	return BatchSendByWebSvc(args, []string{phone}, content)
}
