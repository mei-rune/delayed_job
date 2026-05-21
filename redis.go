package delayed_job

import (
	"bytes"
	"context"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

var redisAddress = flag.String("redis.address", "127.0.0.1:36379", "the address of redis")
var redisPassword = flag.String("redis.password", "", "the address of redis")
var redis_error = expvar.NewString("redis")

type redis_request struct {
	c        chan error
	commands [][]string
}

type redis_gateway struct {
	Address   string
	Password  string
	client    *redis.Client
	ctx       context.Context
	c         chan *redis_request
	is_closed int32
	wait      sync.WaitGroup
}

func (self *redis_gateway) isRunning() bool {
	return 0 == atomic.LoadInt32(&self.is_closed)
}

func (self *redis_gateway) Close() error {
	if atomic.CompareAndSwapInt32(&self.is_closed, 0, 1) {
		return nil
	}
	close(self.c)
	self.wait.Wait()
	return nil
}

func (self *redis_gateway) Send(commands [][]string) {
	self.c <- &redis_request{commands: commands}
}

func (self *redis_gateway) Call(commands [][]string) error {
	c := make(chan error)
	self.c <- &redis_request{c: c, commands: commands}
	return <-c
}

func (self *redis_gateway) serve() {
	defer func() {
		atomic.StoreInt32(&self.is_closed, 1)
		self.wait.Done()
		log.Println("redis client is exit.")
	}()

	error_count := uint(0)
	for self.isRunning() {
		self.runOnce(&error_count)
		if error_count > 5 {
			time.Sleep(1 * time.Second)
		}
	}
}

func (self *redis_gateway) runOnce(error_count *uint) {
	defer func() {
		if e := recover(); nil != e {
			var buffer bytes.Buffer
			buffer.WriteString(fmt.Sprintf("[panic]%v", e))
			for i := 1; ; i++ {
				_, file, line, ok := runtime.Caller(i)
				if !ok {
					break
				}
				buffer.WriteString(fmt.Sprintf("    %s:%d\r\n", file, line))
			}
			msg := buffer.String()
			redis_error.Set(msg)
			log.Println(msg)
		}
	}()

	self.ctx = context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:         self.Address,
		Password:     self.Password,
		DialTimeout:  1 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})

	status := rdb.Ping(self.ctx)
	if status.Err() != nil {
		err := status.Err()
		msg := fmt.Sprintf("[redis] connect to '%s' failed, %v", self.Address, err)
		redis_error.Set(msg)
		*error_count++
		if *error_count < 5 {
			log.Println(msg)
		} else if 0 == (*error_count % 10) {
			log.Println(msg)

			for {
				select {
				case req, ok := <-self.c:
					if !ok {
						return
					}
					if nil != req && nil != req.c {
						select {
						case req.c <- err:
						default:
						}
					}
				default:
					return
				}
			}
		}
		rdb.Close()
		return
	}

	*error_count = 0
	redis_error.Set("")
	self.client = rdb

	for self.isRunning() {
		req, ok := <-self.c
		if !ok {
			break
		}
		if nil == req {
			continue
		}

		if nil == req.commands {
			if nil != req.c {
				req.c <- nil
			}
		}

		e := self.execute(req.commands)
		if nil != req.c {
			req.c <- e
		}
		if nil != e {
			redis_error.Set(e.Error())
			fmt.Println("[redis_gateway]", e)
			break
		}
	}

	rdb.Close()
}

func (self *redis_gateway) execute(commands [][]string) error {
	switch len(commands) {
	case 0:
		return nil
	case 1:
		e := self.redis_do(commands[0])
		if nil != e {
			return errors.New("execute `" + strings.Join(commands[0], " ") + "` failed, " + e.Error())
		}
		return nil
	default:
		pipe := self.client.Pipeline()

		for _, command := range commands {
			args := make([]interface{}, len(command))
			for i, arg := range command {
				args[i] = arg
			}
			pipe.Do(self.ctx, args...)
		}

		_, e := pipe.Exec(self.ctx)
		if nil != e {
			return e
		}
		return nil
	}
}

func (self *redis_gateway) redis_do(cmd []string) error {
	if len(cmd) == 0 {
		return nil
	}
	args := make([]interface{}, len(cmd))
	for i, arg := range cmd {
		args[i] = arg
	}

	_, err := self.client.Do(self.ctx, args...).Result()
	return err
}

func newRedis(address, password string) (*redis_gateway, error) {
	client := &redis_gateway{Address: address, Password: password, c: make(chan *redis_request, 3000)}
	go client.serve()
	client.wait.Add(1)
	return client, nil
}
