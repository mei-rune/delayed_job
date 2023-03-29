package delayed_job

import (
	"errors"
	"flag"
)

var (
	smsExecBatchSupport  string
	smsExecWorkDirectory string
	smsExecCommand       string
	smsExecArguments     string
	smsExecPrompt        string
)

func init() {
	flag.StringVar(&smsExecBatchSupport, "sms.exec.batch_support", "", "")
	flag.StringVar(&smsExecWorkDirectory, "sms.exec.work_directory", "", "")
	flag.StringVar(&smsExecCommand, "sms.exec.command", "", "")
	flag.StringVar(&smsExecArguments, "sms.exec.arguments", "", "")
	flag.StringVar(&smsExecPrompt, "sms.exec.prompt", "", "")
}

func BatchSendByExec(args interface{}, phones []string, content string) error {
	if len(phones) == 0 {
		return nil
	}

	params := map[string]interface{}{
		"args":    args,
		"phones":  phones,
		"phone":   phones[0],
		"content": content,
	}

  text := readStringWith(args, "sms.exec.command", smsExecCommand)
	command, e := genText(text, params)
	if nil != e {
		return errors.New("failed to merge 'command' with params, " + e.Error())
	}

	var arguments []string


  text = readStringWith(args, "sms.exec.arguments", smsExecArguments)
	if smsExecArguments != "" {
		text, e := genText(smsExecArguments, params)
		if nil != e {
			return errors.New("failed to merge 'arguments' with params, " + e.Error())
		}
		arguments = SplitLines(text)
	}

	handler := &execHandler{
		work_directory: readStringWith(args, "sms.exec.work_directory", smsExecWorkDirectory),
		prompt:         readStringWith(args, "sms.exec.prompt", smsExecPrompt),
		command:        command,
		arguments:      arguments,
		// environments: environments,
	}

	return handler.Perform()
}

func SendByExec(args interface{}, phone, content string) error {
	return BatchSendByExec(args, []string{phone}, content)
}
