package delayed_job

import (
	"bytes"
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

	"github.com/garyburd/redigo/redis"
)

var redisAddress = flag.String("redis", "127.0.0.1:6379", "the address of redis")
var redis_error = expvar.NewString("redis")

type redis_request struct {
	c        chan error
	commands [][]string
}

type redis_gateway struct {
	Address   string
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
	}
}

func (self *redis_gateway) runOnce(error_count *uint) {
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
			redis_error.Set(msg)
			log.Println(msg)
		}
	}()

	c, err := redis.DialTimeout("tcp", self.Address, 1*time.Second, 1*time.Second, 1*time.Second)
	if err != nil {
		msg := fmt.Sprintf("[redis] connect to '%s' failed, %v", self.Address, err)
		redis_error.Set(msg)
		log.Println(msg)

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

		return
	}

	*error_count = 0
	redis_error.Set("")

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

		e := self.execute(c, req.commands)
		if nil != req.c {
			req.c <- e
		}
		if nil != e {
			redis_error.Set(e.Error())
			break
		}
	}
}

func (self *redis_gateway) execute(c redis.Conn, commands [][]string) error {
	switch len(commands) {
	case 0:
		return nil
	case 1:
		e := self.redis_do(c, commands[0])
		if nil != e {
			return errors.New("execute `" + strings.Join(commands[0], " ") + "` failed, " + e.Error())
		}
		return nil
	default:
		for _, command := range commands {
			e := self.redis_send(c, command)
			if nil != e {
				return errors.New("execute `" + strings.Join(command, " ") + "` failed, " + e.Error())
			}
		}

		e := c.Flush()
		if nil != e {
			return e
		}

		for i := 0; i < len(commands); i++ {
			_, e = c.Receive()
			if nil != e {
				return errors.New("execute `" + strings.Join(commands[i], " ") + "` failed, " + e.Error())
			}
		}
		return nil
	}
}

func (self *redis_gateway) redis_send(c redis.Conn, cmd []string) (err error) {
	switch len(cmd) {
	case 1:
		err = c.Send(cmd[0])
	case 2:
		err = c.Send(cmd[0], cmd[1])
	case 3:
		err = c.Send(cmd[0], cmd[1], cmd[2])
	case 4:
		err = c.Send(cmd[0], cmd[1], cmd[2], cmd[3])
	case 5:
		err = c.Send(cmd[0], cmd[1], cmd[2], cmd[3], cmd[4])
	case 6:
		err = c.Send(cmd[0], cmd[1], cmd[2], cmd[3], cmd[4], cmd[5])
	default:
		err = errors.New("argument length is error.")
	}
	return err
}

func (self *redis_gateway) redis_do(c redis.Conn, cmd []string) (err error) {
	switch len(cmd) {
	case 1:
		_, err = c.Do(cmd[0])
	case 2:
		_, err = c.Do(cmd[0], cmd[1])
	case 3:
		_, err = c.Do(cmd[0], cmd[1], cmd[2])
	case 4:
		_, err = c.Do(cmd[0], cmd[1], cmd[2], cmd[3])
	case 5:
		_, err = c.Do(cmd[0], cmd[1], cmd[2], cmd[3], cmd[4])
	case 6:
		_, err = c.Do(cmd[0], cmd[1], cmd[2], cmd[3], cmd[4], cmd[5])
	default:
		err = errors.New("argument length is error.")
	}
	return err
}

func newRedis(address string) (*redis_gateway, error) {
	client := &redis_gateway{Address: address, c: make(chan *redis_request, 3000)}
	go client.serve()
	client.wait.Add(1)
	return client, nil
}
