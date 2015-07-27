package delayed_job

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
)

func clearRedis(t *testing.T, c redis.Conn, key string) {
	reply, err := c.Do("DEL", key)
	_, err = redis.Int(reply, err)
	if nil != err {
		t.Logf("DEL %s failed, %v", key, err)
	}
}

func redisTest(t *testing.T, cb func(client *redis_gateway, c redis.Conn)) {
	go func() {
		http.ListenAndServe(":7890", nil)
	}()
	redis_client, err := newRedis(*redisAddress)
	if nil != err {
		t.Error(err)
		return
	}
	defer redis_client.Close()

	c, err := redis.DialTimeout("tcp", *redisAddress, 0, 1*time.Second, 1*time.Second)
	if err != nil {
		t.Errorf("[redis] connect to '%s' failed, %v", *redisAddress, err)
		return
	}
	defer c.Close()

	for i := 0; i < 10; i++ {
		clearRedis(t, c, fmt.Sprintf("a%v", i))
	}

	cb(redis_client, c)
}

func checkResult(t *testing.T, c redis.Conn, cmd, key, excepted string) {
	reply, err := c.Do(cmd, key)
	s, err := redis.String(reply, err)
	if nil != err {
		t.Errorf("GET %s failed, %v", key, err)
	} else if excepted != s {
		t.Errorf("check %s failed, actual is %v, excepted is %v", key, reply, excepted)
	}
}

func TestRedis(t *testing.T) {
	redisTest(t, func(redis_client *redis_gateway, c redis.Conn) {
		redis_client.c <- &redis_request{commands: [][]string{{"SET", "a1", "1223"}}}
		redis_client.c <- &redis_request{commands: [][]string{{"SET", "a2", "1224"}}}
		redis_client.Send([][]string{{"SET", "a3", "1225"}})
		redis_client.Send([][]string{{"SET", "a4", "1226"}})
		redis_client.Send([][]string{{"SET", "a5", "1227"}})

		time.Sleep(2 * time.Second)

		checkResult(t, c, "GET", "a1", "1223")
		checkResult(t, c, "GET", "a2", "1224")
		checkResult(t, c, "GET", "a3", "1225")
		checkResult(t, c, "GET", "a4", "1226")
		checkResult(t, c, "GET", "a5", "1227")
	})
}

func TestRedisEmpty(t *testing.T) {
	redisTest(t, func(redis_client *redis_gateway, c redis.Conn) {
		redis_client.c <- &redis_request{commands: [][]string{}}
		redis_client.Send([][]string{{}})

		redis_client.c <- &redis_request{commands: [][]string{{"SET", "a1", "1223"}}}
		redis_client.c <- &redis_request{commands: [][]string{{"SET", "a2", "1224"}}}
		redis_client.Send([][]string{{"SET", "a3", "1225"}})
		redis_client.Send([][]string{{"SET", "a4", "1226"}})
		redis_client.Send([][]string{{"SET", "a5", "1227"}})

		time.Sleep(2 * time.Second)

		checkResult(t, c, "GET", "a1", "1223")
		checkResult(t, c, "GET", "a2", "1224")
		checkResult(t, c, "GET", "a3", "1225")
		checkResult(t, c, "GET", "a4", "1226")
		checkResult(t, c, "GET", "a5", "1227")
	})
}

func TestRedisConnectFailed(t *testing.T) {
	redis_client, err := newRedis("127.0.0.1:3")
	if nil != err {
		t.Error(err)
		return
	}
	defer redis_client.Close()

	e := redis_client.Call([][]string{{}})
	if nil == e {
		t.Error("excepted is error, actual is ok")
		return
	}
	if !strings.Contains(e.Error(), "127.0.0.1:3") {
		t.Error(e)
	}
	t.Log(e)
}
