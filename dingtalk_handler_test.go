package delayed_job

// var (
// 	weixin_corp_id     = flag.String("weixin_corp_id", "", "")
// 	weixin_corp_secret = flag.String("weixin_corp_secret", "", "")
// 	weixin_target_type = flag.String("weixin_target_type", "user", "")
// 	weixin_targets     = flag.String("weixin_targets", "", "")
// 	weixin_agent_id    = flag.String("weixin_agent_id", "1", "")
// )

// func TestDingtalkHandler(t *testing.T) {
// 	if "" == *weixin_corp_id {
// 		t.Skip("weixin is skipped.")
// 	}

// 	handler, e := newWeixinHandler(nil, map[string]interface{}{"type": "weixin",
// 		"corp_id":     *weixin_corp_id,
// 		"corp_secret": *weixin_corp_secret,
// 		"target_type": *weixin_target_type,
// 		"targets":     *weixin_targets,
// 		"content":     "this is test message.",
// 		"agent_id":    *weixin_agent_id})
// 	if nil != e {
// 		t.Error(e)
// 		// if e.Error() != test.excepted_error {
// 		// 	t.Error(e)
// 		// }
// 		return
// 	}

// 	e = handler.Perform()
// 	if nil != e {
// 		t.Error(e)
// 		// if e.Error() != test.excepted_error {
// 		// 	t.Error(e)
// 		// }
// 		return
// 	}
// }

// func TestRunWeixin(t *testing.T) {
// 	t.Skip("===")

// 	e := Main(":0", "backend")
// 	if nil != e {
// 		t.Error(e)
// 		return
// 	}
// 	w, e := newWorker(map[string]interface{}{})
// 	if nil != e {
// 		t.Error(e)
// 		return
// 	}
// 	defer w.innerClose()

// 	w.start()
// 	defer w.Close()

// 	func(w *worker, backend *dbBackend) {
// 		e := backend.enqueue(1, 0, 1, "aa", time.Time{}, map[string]interface{}{"type": "test"})
// 		if nil != e {
// 			t.Error(e)
// 			return
// 		}

// 		select {
// 		case <-test_chan:
// 			return
// 		case <-time.After(2 * time.Second):
// 			t.Error("not recv")
// 		}
// 	}(w, w.backend)
// }
