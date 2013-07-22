package delayed_job

import (
	"bytes"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var redisAddress = flag.String("redis", "127.0.0.1:6379", "the address of redis")
var redis_error = expvar.NewString("redis")

type redis_request struct {
	c        chan error
	commands [][]string
}

type redis_keeper struct {
	Address string
	c       chan *redis_request
	status  int32
	wait    sync.WaitGroup
}

func (self *redis_keeper) isRunning() bool {
	return 1 == atomic.LoadInt32(&self.status)
}

func (self *redis_keeper) serve() {
	defer func() {
		close(self.c)
		self.wait.Done()
		log.Println("redis client is exit.")
	}()

	ticker := time.NewTicker(1 * time.Second)

	go func() {
		defer func() {
			if o := recover(); nil != o {
				log.Println("[panic]", o)
			}
			self.wait.Done()
			ticker.Stop()
		}()

		for self.isRunning() {
			<-ticker.C
			self.c <- nil
		}
	}()

	self.wait.Add(1)

	for self.isRunning() {
		self.runOnce()
	}
}

func (self *redis_keeper) runOnce() {
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

	c, err := redis.DialTimeout("tcp", self.Address, 0, 1*time.Second, 1*time.Second)
	if err != nil {
		msg := fmt.Sprintf("[redis] connect to '%s' failed, %v", self.Address, err)
		redis_error.Set(msg)
		log.Println(msg)
		return
	}

	redis_error.Set("")

	for self.isRunning() {
		req := <-self.c
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

func (self *redis_keeper) execute(c redis.Conn, commands [][]string) error {
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

func (self *redis_keeper) redis_send(c redis.Conn, cmd []string) (err error) {
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

func (self *redis_keeper) redis_do(c redis.Conn, cmd []string) (err error) {
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

func (self *redis_keeper) Close() {
	atomic.StoreInt32(&self.status, 0)
	self.wait.Wait()
}

func (self *redis_keeper) Send(commands [][]string) {
	self.c <- &redis_request{commands: commands}
}

func (self *redis_keeper) Call(commands [][]string) error {
	c := make(chan error)
	self.c <- &redis_request{c: c, commands: commands}
	return <-c
}

func newRedis(address string) (*redis_keeper, error) {
	client := &redis_keeper{Address: address, c: make(chan *redis_request, 3000), status: 1}
	go client.serve()
	client.wait.Add(1)
	return client, nil
}