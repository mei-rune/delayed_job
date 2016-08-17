package delayed_job

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var (
	MaxJobTimeout = 15 * time.Minute
	sequeuce_lock sync.Mutex
	sequence_id   = uint64(0)
	sequeuce_seed = strconv.FormatInt(time.Now().Unix(), 10) + "_"
)

func init() {
	flag.DurationVar(&MaxJobTimeout, "max_job_timeout", 15*time.Minute, "the max time of execute job.")
}

func generate_id() string {
	sequeuce_lock.Lock()
	defer sequeuce_lock.Unlock()
	sequence_id += 1
	if sequence_id >= 18446744073709551610 {
		sequence_id = 0
		sequeuce_seed = strconv.FormatInt(time.Now().Unix(), 10) + "_"
	}
	return sequeuce_seed + strconv.FormatUint(sequence_id, 10)
}

type Job struct {
	backend *dbBackend

	id           int64
	priority     int
	repeat_count int
	attempts     int
	max_attempts int
	queue        string
	handler      string
	handler_id   string
	last_error   string
	run_at       time.Time
	failed_at    time.Time
	locked_at    time.Time
	locked_by    string
	created_at   time.Time
	updated_at   time.Time

	changed_attributes map[string]interface{}
	handler_attributes map[string]interface{}
	handler_object     Handler
}

func createJobFromMap(backend *dbBackend, args map[string]interface{}) (*Job, error) {
	priority := intWithDefault(args, "priority", *default_priority)
	repeat_count := intWithDefault(args, "repeat_count", 0)
	max_attempts := intWithDefault(args, "max_attempts", 0)
	queue := stringWithDefault(args, "queue", *default_queue_name)
	run_at := timeWithDefault(args, "run_at", backend.db_time_now())
	handler_o, ok := args["handler"]
	if !ok {
		return nil, errors.New("'Handler' is missing.")
	}

	handler, ok := handler_o.(map[string]interface{})
	if !ok {
		return nil, errors.New("'Handler' is not a map[string]interface{}.")
	}

	is_valid_rule := boolWithDefault(args, "is_valid_rule", true)
	return newJob(backend, priority, repeat_count, max_attempts, queue, run_at, handler, is_valid_rule)
}

func newJob(backend *dbBackend, priority, repeat_count, max_attempts int, queue string, run_at time.Time, args map[string]interface{}, is_valid_payload_object bool) (*Job, error) {
	id := stringWithDefault(args, "_uid", "")
	if 0 == len(id) {
		id = generate_id()
	}

	s, e := json.MarshalIndent(args, "", "  ")
	if nil != e {
		return nil, deserializationError(e)
	}

	j := &Job{backend: backend,
		priority:           priority,
		repeat_count:       repeat_count,
		max_attempts:       max_attempts,
		queue:              queue,
		handler:            string(s),
		handler_id:         id,
		run_at:             run_at,
		handler_attributes: args}

	if is_valid_payload_object {
		_, e = j.payload_object()
		if nil != e {
			return nil, e
		}
	}
	return j, nil
}

func (self *Job) isFailed() bool {
	return self.failed_at.IsZero()
}

func (self *Job) name() string {
	options, e := self.attributes()
	if nil == e && nil != options {
		if v, ok := options["display_name"]; ok {
			return fmt.Sprint(v)
		}
		if v, ok := options["name"]; ok {
			return fmt.Sprint(v)
		}
	}
	return "unknow"
}

func (self *Job) attributes() (map[string]interface{}, error) {
	if nil != self.handler_attributes {
		return self.handler_attributes, nil
	}
	if 0 == len(self.handler) {
		return nil, deserializationError(errors.New("handle is empty"))
	}
	e := json.Unmarshal([]byte(self.handler), &self.handler_attributes)
	if nil != e {
		return nil, deserializationError(e)
	}
	return self.handler_attributes, nil
}

func (self *Job) payload_object() (Handler, error) {
	if nil != self.handler_object {
		return self.handler_object, nil
	}
	options, e := self.attributes()
	if nil != e {
		return nil, e
	}

	if nil == self.backend {
		return nil, errors.New("the backend of job is nil")
	}

	self.handler_object, e = newHandler(self.backend.ctx, options)
	if nil != e {
		return nil, errors.New("create job handler failed, " + e.Error())
	}
	return self.handler_object, nil
}

var ErrTimeout = errors.New("time out")

func (self *Job) invokeJob() error {
	job, e := self.payload_object()
	if nil != e {
		return e
	}
	ch := make(chan error, 1)
	go func() {
		defer func() {
			if e := recover(); nil != e {
				var buffer bytes.Buffer
				buffer.WriteString(fmt.Sprintf("[panic]%v", e))
				for i := 1; ; i += 1 {
					_, file, line, ok := runtime.Caller(i)
					if !ok {
						break
					}
					buffer.WriteString(fmt.Sprintf("    %s:%d\r\n", file, line))
				}
				msg := buffer.String()
				ch <- errors.New(msg)
			}
		}()

		ch <- job.Perform()
	}()

	timer := time.NewTimer(self.execTimeout())
	select {
	case err := <-ch:
		timer.Stop()
		return err
	case <-timer.C:
		return ErrTimeout
	}
}

func (self *Job) needReschedule() (time.Time, bool) {
	if self.repeat_count <= 0 {
		return time.Time{}, false
	}

	var duration time.Duration

	options, e := self.attributes()
	if nil != e {
		goto default_duration
	}

	duration = durationWithDefault(options, "repeat_interval", 0)
	if duration < 5*time.Second {
		goto default_duration
	}
	return self.backend.db_time_now().Add(duration), true

default_duration:
	duration = time.Duration(self.attempts*10) * time.Minute
	return self.backend.db_time_now().Add(duration).Add(5 + time.Second), true
}

func (self *Job) reschedule_at() time.Time {
	var duration time.Duration

	options, e := self.attributes()
	if nil != e {
		goto default_duration
	}

	duration = durationWithDefault(options, "try_interval", 0)
	if duration < 5*time.Second {
		goto default_duration
	}
	return self.backend.db_time_now().Add(duration)

default_duration:
	duration = time.Duration(self.attempts*10) * time.Second
	return self.backend.db_time_now().Add(duration).Add(5 + time.Second)
}

func (self *Job) get_max_attempts() int {
	if self.max_attempts > 0 {
		return self.max_attempts
	}
	options, e := self.attributes()
	if nil != e {
		return -1
	}

	if m, ok := options["max_attempts"]; ok {
		i, e := strconv.ParseInt(fmt.Sprint(m), 10, 0)
		if nil == e {
			return int(i)
		}
	}
	return -1
}

func (self *Job) execTimeout() time.Duration {
	options, e := self.attributes()
	if nil != e {
		return MaxJobTimeout
	}

	if m, ok := options["exec_timeout"]; ok {
		i, e := time.ParseDuration(fmt.Sprint(m))
		if nil == e {
			return i
		}
		log.Println("[warn] [", self.id, self.name(), "] parse exec_timeout(", m, ") failed,", e)
	}
	return MaxJobTimeout
}

func stringifiedHander(params map[string]interface{}) error {
	handler, ok := params["@handler"]
	if !ok {
		return nil
	}

	if nil == handler {
		return nil
	}

	if _, ok := handler.(string); ok {
		return nil
	}

	bs, e := json.MarshalIndent(handler, "", "  ")
	if nil != e {
		return e
	}
	params["@handler"] = string(bs)
	return nil
}

func (self *Job) will_update_attributes() map[string]interface{} {
	if nil == self.changed_attributes {
		self.changed_attributes = make(map[string]interface{}, 8)
	}

	if update, ok := self.handler_object.(Updater); ok {
		if nil != self.handler_attributes {
			update.UpdatePayloadObject(self.handler_attributes)
			self.changed_attributes["@handler"] = self.handler_attributes
		}
	}

	return self.changed_attributes
}

func (self *Job) rescheduleIt(next_time time.Time, err string) error {
	if len(err) > 2000 {
		err = err[:1900] + "\r\n===========================\r\n**error message is overflow."
	}

	self.attempts += 1
	self.run_at = next_time
	self.locked_at = time.Time{}
	self.locked_by = ""
	self.last_error = err

	changed := self.will_update_attributes()
	e := stringifiedHander(changed)
	if nil != e {
		return e
	}
	changed["@attempts"] = self.attempts
	changed["@run_at"] = next_time
	changed["@locked_at"] = nil
	changed["@locked_by"] = nil
	changed["@last_error"] = err
	if "" == err {
		changed["@repeat_count"] = self.repeat_count - 1
	}

	e = self.backend.update(self.id, changed)
	self.changed_attributes = nil
	return e
}

func (self *Job) failIt(err string) error {
	if len(err) > 2000 {
		err = err[:1900] + "\r\n===========================\r\n**error message is overflow."
	}
	now := self.backend.db_time_now()
	self.failed_at = now
	self.last_error = err
	return self.backend.update(self.id, map[string]interface{}{"@failed_at": now, "@last_error": err})
}

func (self *Job) destroyIt() error {
	return self.backend.destroy(self.id)
}
