package delayed_job

import (
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
	default_priority            = flag.Int("default_priority", 0, "the default priority of job")
	default_queue_name          = flag.String("default_queue_name", "", "the default queue name")
	delay_jobs                  = flag.Bool("delay_jobs", true, "can delay job")
	name_prefix                 = flag.String("name_prefix", "tpt_worker", "the prefix of worker name")
	default_min_priority        = flag.Int("min_priority", -1, "the min priority")
	default_max_priority        = flag.Int("max_priority", -1, "the max priority")
	default_max_attempts        = flag.Int("max_attempts", 25, "the max attempts")
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
	backend, e := newBackend(*db_drv, *db_url, ctx)
	if nil != e {
		return nil, e
	}

	redis_client, e := newRedis(*redisAddress)
	if nil != e {
		return nil, e
	}

	ctx["redis"] = redis_client
	ctx["backend"] = backend

	w := &worker{ctx: ctx,
		backend:  backend,
		shutdown: make(chan int)}
	w.initialize(options)

	w.closes = append(w.closes, redis_client)
	w.closes = append(w.closes, backend)
	return w, nil
}

func (w *worker) RunForever() {
	w.wait.Add(1)
	w.serve()
}

func (w *worker) start() {
	w.wait.Add(1)
	go w.serve()
}

func (self *worker) Close() {
	self.shutdown <- 0
	self.wait.Wait()

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

func (self *worker) serve() {
	defer func() {
		self.wait.Done()
	}()

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

	e = job.destroyIt()
	self.job_say(job, "COMPLETED after ", time.Now().Sub(now))
	return true, e // did work
}

func (self *worker) failed(job *Job, e error) error {
	if self.destroy_failed_jobs {
		return job.destroyIt()
	} else {
		return job.failIt(e.Error())
	}
}

func (self *worker) job_say(job *Job, text ...interface{}) {
	args := make([]interface{}, 0, 3+len(text))
	args = append(args, "Job ", job.name(), " (id=", job.id, ") ")
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
	job_max_attempts := job.max_attempts()
	if -1 == job_max_attempts {
		return self.max_attempts
	}
	return job_max_attempts
}

func (self *worker) handle_failed_job(job *Job, e error) error {
	self.job_say(job, "FAILED (", job.attempts, " prior attempts) with ", e)
	return self.reschedule(job, time.Time{}, e)
}

// Reschedule the job in the future (when a job fails).
// Uses an exponential scale depending on the number of failed attempts.
func (self *worker) reschedule(job *Job, next_time time.Time, e error) error {
	attempts := job.attempts + 1
	if attempts < self.get_max_attempts(job) {
		if next_time.IsZero() {
			next_time = job.reschedule_at()
		}
		return job.rescheduleIt(next_time, e.Error())
	} else {
		self.job_say(job, "REMOVED permanently because of ", job.attempts, " consecutive failures")
		return self.failed(job, e)
	}
}

type TestWorker struct {
	*worker
}

func (self *TestWorker) WorkOff(num int) (int, int, error) {
	return self.work_off(num)
}

func WorkTest(t *testing.T, cb func(w *TestWorker)) {
	w, e := newWorker(map[string]interface{}{})
	if nil != e {
		t.Error(e)
		return
	}

	_, e = w.backend.db.Exec(`
DROP TABLE IF EXISTS ` + *table_name + `;

CREATE TABLE IF NOT EXISTS ` + *table_name + ` (
  id                BIGSERIAL  PRIMARY KEY,
  priority          int DEFAULT 0,
  attempts          int DEFAULT 0,
  queue             varchar(200),
  handler           text  NOT NULL,
  handler_id        varchar(200),
  last_error        varchar(2000),
  run_at            timestamp with time zone,
  locked_at         timestamp with time zone,
  failed_at         timestamp with time zone,
  locked_by         varchar(200),
  created_at        timestamp with time zone  NOT NULL,
  updated_at        timestamp with time zone NOT NULL
);`)
	if nil != e {
		t.Error(e)
		return
	}

	cb(&TestWorker{w})
}
