package delayed_job

import (
	"bytes"
	"encoding/json"
	"errors"
	"expvar"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	dump_job                    = false
	default_priority            = flag.Int("default_priority", 0, "the default priority of job")
	default_queue_name          = flag.String("default_queue_name", "", "the default queue name")
	delay_jobs                  = flag.Bool("delay_jobs", true, "can delay job")
	name_prefix                 = flag.String("name_prefix", "tpt_worker", "the prefix of worker name")
	default_min_priority        = flag.Int("min_priority", -1, "the min priority")
	default_max_priority        = flag.Int("max_priority", -1, "the max priority")
	default_max_attempts        = flag.Int("max_attempts", 3, "the max attempts")
	default_max_run_time        = flag.Duration("max_run_time", 1*time.Minute, "the max run time")
	default_sleep_delay         = flag.Duration("sleep_delay", 10*time.Second, "the sleep delay")
	default_read_ahead          = flag.Int("read_ahead", 10, "the read ahead")
	default_queues              = flag.String("queues", "", "the queue name of worker")
	default_exit_on_complete    = flag.Bool("exit_on_complete", false, "exit worker while jobs complete")
	default_destroy_failed_jobs = flag.Bool("destroy_failed_jobs", false, "the failed jobs are destroyed after too many attempts")
)

var work_error = expvar.NewString("worker")
var jobs_is_empty = errors.New("jobs is empty in the db")

func deserializationError(e error) error {
	return errors.New("[deserialization]" + e.Error())
}

func isDeserializationError(e error) bool {
	return strings.Contains(e.Error(), "[deserialization]")
}

type worker struct {
	ctx     map[string]interface{}
	backend *dbBackend

	min_priority int
	max_priority int
	max_attempts int
	max_run_time time.Duration
	sleep_delay  time.Duration
	queues       []string
	read_ahead   int

	// By default failed jobs are destroyed after too many attempts. If you want to keep them around
	// (perhaps to inspect the reason for the failure), set this to false.
	destroy_failed_jobs bool
	exit_on_complete    bool

	name string

	shutdown chan int
	wait     sync.WaitGroup

	closes []io.Closer
}

func newWorker(options map[string]interface{}) (*worker, error) {
	ctx := map[string]interface{}{}

	dbDrv := stringWithDefault(options, "db_drv", "")
	dbURL := stringWithDefault(options, "db_url", "")

	backend, e := newBackend(dbDrv, dbURL, ctx)
	if nil != e {
		return nil, e
	}

	redis_client, e := newRedis(*redisAddress, *redisPassword)
	if nil != e {
		return nil, e
	}

	ctx["redis"] = redis_client
	ctx["backend"] = backend

	w := &worker{
		ctx:      ctx,
		backend:  backend,
		shutdown: make(chan int),
	}
	w.initialize(options)

	w.closes = append(w.closes, redis_client)
	w.closes = append(w.closes, backend)
	return w, nil
}

func (w *worker) RunForever() {
	w.serve(false)
}

func (w *worker) start() {
	w.wait.Add(1)
	go w.serve(true)
}

func (self *worker) Close() error {
	close(self.shutdown)
	self.wait.Wait()

	self.innerClose()
	return nil
}

func (self *worker) innerClose() {
	if nil != self.closes {
		for _, cl := range self.closes {
			cl.Close()
		}
	}
}

func (self *worker) initialize(options map[string]interface{}) {
	self.min_priority = intWithDefault(options, "min_priority", *default_min_priority)
	self.max_priority = intWithDefault(options, "max_priority", *default_max_priority)
	self.max_attempts = intWithDefault(options, "max_attempts", *default_max_attempts)
	self.max_run_time = durationWithDefault(options, "max_run_time", *default_max_run_time)
	self.sleep_delay = durationWithDefault(options, "sleep_delay", *default_sleep_delay)
	self.read_ahead = intWithDefault(options, "read_ahead", *default_read_ahead)
	if 0 == len(*default_queues) {
		self.queues = stringsWithDefault(options, "queues", ",", nil)
	} else {
		self.queues = stringsWithDefault(options, "queues", ",", strings.Split(*default_queues, ","))
	}

	self.exit_on_complete = boolWithDefault(options, "exit_on_complete", *default_exit_on_complete)
	self.destroy_failed_jobs = boolWithDefault(options, "destroy_failed_jobs", *default_destroy_failed_jobs)

	// Every worker has a unique name which by default is the pid of the process. There are some
	// advantages to overriding this with something which survives worker restarts:  Workers can
	// safely resume working on tasks which are locked by themselves. The worker will assume that
	// it crashed before.
	self.name = *name_prefix + "_pid:" + strconv.FormatInt(int64(os.Getpid()), 10)
}

// func (self *worker) reset() {
// 	self.sleep_delay = DEFAULT_SLEEP_DELAY
// 	self.max_attempts = DEFAULT_MAX_ATTEMPTS
// 	self.max_run_time = DEFAULT_MAX_RUN_TIME
// 	self.queues = []string{}
// 	self.read_ahead = DEFAULT_READ_AHEAD
// 	self.destroy_failed_jobs = true
// }

func (self *worker) before_fork() {
	// unless @files_to_reopen
	//   @files_to_reopen = []
	//   ObjectSpace.each_object(File) do |file|
	//     @files_to_reopen << file unless file.closed?
	//   }
	// }

	//backend.before_fork
}

func (self *worker) after_fork() {
	// // Re-open file handles
	// @files_to_reopen.each do |file|
	//   begin
	//     file.reopen file.path, "a+"
	//     file.sync = true
	//   rescue ::Exception
	//   }
	// }

	//backend.after_fork()
}

func (self *worker) lifecycle() {
	//@lifecycle ||= Delayed::Lifecycle.new
}

func (self *worker) serve(in_goroutine bool) {
	if in_goroutine {
		defer self.wait.Done()
	}

	self.say("Starting job worker")

	//self.before_execute()
	//defer self.after_execute()

	is_running := true
	for is_running {
		for is_running {
			now := time.Now()

			success, failure, e := self.work_off(10)
			if nil != e {
				log.Println(e)
				work_error.Set(e.Error())
				break
			}

			if 0 == success {
				if self.exit_on_complete {
					self.say("No more jobs available. Exiting")
				}
				break
			} else {
				self.say(success, "jobs processed at ", float64(success)/time.Now().Sub(now).Seconds(), " j/s, ", failure, " failed")
			}

			work_error.Set("")
		}

		select {
		case <-self.shutdown:
			is_running = false
		case <-time.After(self.sleep_delay):
		}
	}
}

// Do num jobs and return stats on success/failure.
// Exit early if interrupted.
func (self *worker) work_off(num int) (int, int, error) {
	success, failure := 0, 0

	for i := 0; i < num; i++ {
		ok, e := self.reserve_and_run_one_job()
		if nil != e {
			if jobs_is_empty == e {
				return success, failure, nil
			}
			return success, failure, e
		}

		if ok {
			success += 1
		} else {
			failure += 1
		}
	}

	return success, failure, nil
}

// Run the next job we can get an exclusive lock on.
// If no jobs are left we return nil
func (self *worker) reserve_and_run_one_job() (bool, error) {
	job, e := self.backend.reserve(self)
	if nil != e {
		return false, e
	}

	if nil == job {
		return false, jobs_is_empty
	}

	return self.run(job)
}

func (self *worker) run(job *Job) (bool, error) {
	self.job_say(job, "RUNNING")
	now := time.Now()
	e := job.invokeJob()
	if nil != e {
		if isDeserializationError(e) {
			self.job_say(job, "FAILED (", job.attempts, " prior attempts) with ", e)
			e = self.failed(job, e)
		} else {
			e = self.handle_failed_job(job, e)
		}
		return false, e // work failed
	}

	if next_time, need := job.needReschedule(); need {
		e = job.rescheduleIt(next_time, "")
		return true, e
	}

	e = job.destroyIt()
	self.job_say(job, "COMPLETED after ", time.Now().Sub(now))
	return true, e // did work
}

func (self *worker) failed(job *Job, e error) error {
	if self.destroy_failed_jobs {
		self.job_say(job, "REMOVED permanently because of attempts = ", job.attempts, "and max_attempts = ", self.get_max_attempts(job), " consecutive failures")
		return job.destroyIt()
	} else {
		self.job_say(job, "STOPPED permanently because of attempts = ", job.attempts, "and max_attempts = ", self.get_max_attempts(job), " consecutive failures")
		return job.failIt(e.Error())
	}
}

func (self *worker) job_say(job *Job, text ...interface{}) {
	args := make([]interface{}, 0, 3+len(text))
	if dump_job {
		txt, _ := json.Marshal(job.handler_attributes)
		args = append(args, "Job ", job.name(), string(txt))
	} else {
		args = append(args, "Job ", job.name(), " (id=", job.id, ") ")
	}
	args = append(args, text...)
	self.say(args...)
}

func (self *worker) say(text ...interface{}) {
	args := make([]interface{}, 0, 3+len(text))
	args = append(args, "[Worker(", self.name, ")] ")
	args = append(args, text...)
	log.Println(args...)
}

func (self *worker) get_max_attempts(job *Job) int {
	job_max_attempts := job.get_max_attempts()
	if job_max_attempts <= 0 {
		return self.max_attempts
	}
	return job_max_attempts
}

func (self *worker) handle_failed_job(job *Job, e error) error {
	self.job_say(job, "FAILED (", job.attempts, " attempts and", self.get_max_attempts(job), " max_attempts) with ", e)
	return self.reschedule(job, time.Time{}, e)
}

// Reschedule the job in the future (when a job fails).
// Uses an exponential scale depending on the number of failed attempts.
func (self *worker) reschedule(job *Job, next_time time.Time, e error) error {
	attempts := job.attempts + 1
	if attempts <= self.get_max_attempts(job) {
		if next_time.IsZero() {
			next_time = job.reschedule_at()
		}
		return job.rescheduleIt(next_time, e.Error())
	} else {
		return self.failed(job, e)
	}
}

type TestWorker struct {
	*worker
	Buffer bytes.Buffer
}

func (self *TestWorker) WorkOff(num int) (int, int, error) {
	dump_job = true
	log.SetOutput(&self.Buffer)
	defer func() {
		log.SetOutput(os.Stderr)
		dump_job = false
	}()

	return self.work_off(num)
}

func WorkTest(t *testing.T, dbDrv, dbURL string, cb func(w *TestWorker)) {
	e := Main("init_db", dbDrv, dbURL, nil)
	if nil != e {
		t.Error(e)
		return
	}
	w, e := newWorker(map[string]interface{}{
		"db_drv": dbDrv,
		"db_url": dbURL,
	})
	if nil != e {
		t.Error(e)
		return
	}
	defer w.innerClose()

	cb(&TestWorker{worker: w})
}
