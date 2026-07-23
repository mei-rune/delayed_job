package delayed_job

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	_ "net/http/pprof"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/tidwall/redcon"
)

var redisSim = flag.Bool("redis_sim", true, "")

var addr = ":6380"

func StartRedis(addr string) (*redcon.Server, int, error) {
	var mu sync.RWMutex
	var items = make(map[string][]byte)
	var ps redcon.PubSub

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, 0, err
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	srv := redcon.NewServer(ln.Addr().String(),
		func(conn redcon.Conn, cmd redcon.Command) {
			switch strings.ToLower(string(cmd.Args[0])) {
			default:
				conn.WriteError("ERR unknown command '" + string(cmd.Args[0]) + "'")
			case "publish":
				// Publish to all pub/sub subscribers and return the number of
				// messages that were sent.
				if len(cmd.Args) != 3 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				count := ps.Publish(string(cmd.Args[1]), string(cmd.Args[2]))
				conn.WriteInt(count)
			case "subscribe", "psubscribe":
				// Subscribe to a pub/sub channel. The `Psubscribe` and
				// `Subscribe` operations will detach the connection from the
				// event handler and manage all network I/O for this connection
				// in the background.
				if len(cmd.Args) < 2 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				command := strings.ToLower(string(cmd.Args[0]))
				for i := 1; i < len(cmd.Args); i++ {
					if command == "psubscribe" {
						ps.Psubscribe(conn, string(cmd.Args[i]))
					} else {
						ps.Subscribe(conn, string(cmd.Args[i]))
					}
				}
			case "detach":
				hconn := conn.Detach()
				log.Printf("connection has been detached")
				go func() {
					defer hconn.Close()
					hconn.WriteString("OK")
					hconn.Flush()
				}()
			case "ping":
				conn.WriteString("PONG")
			case "quit":
				conn.WriteString("OK")
				conn.Close()
			case "set":
				if len(cmd.Args) != 3 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				mu.Lock()
				items[string(cmd.Args[1])] = cmd.Args[2]
				mu.Unlock()
				conn.WriteString("OK")

				fmt.Println("[server] set", string(cmd.Args[1]), cmd.Args[2])
			case "get":
				if len(cmd.Args) != 2 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				mu.RLock()
				val, ok := items[string(cmd.Args[1])]
				mu.RUnlock()

				fmt.Println("[server] get", string(cmd.Args[1]), string(val), ok)
				if !ok {
					conn.WriteNull()
				} else {
					conn.WriteBulk(val)
				}
			case "del":
				if len(cmd.Args) != 2 {
					conn.WriteError("ERR wrong number of arguments for '" + string(cmd.Args[0]) + "' command")
					return
				}
				mu.Lock()
				_, ok := items[string(cmd.Args[1])]
				delete(items, string(cmd.Args[1]))
				mu.Unlock()
				if !ok {
					conn.WriteInt(0)
				} else {
					conn.WriteInt(1)
				}
			case "config":
				// This simple (blank) response is only here to allow for the
				// redis-benchmark command to work with this example.
				conn.WriteArray(2)
				conn.WriteBulk(cmd.Args[2])
				conn.WriteBulkString("")
			}
		},
		func(conn redcon.Conn) bool {
			// Use this function to accept or deny the connection.
			log.Printf("[redis-sim] accept: %s", conn.RemoteAddr())
			return true
		},
		func(conn redcon.Conn, err error) {
			// This is called when the connection has been closed
			log.Printf("[redis-sim] closed: %s, err: %v", conn.RemoteAddr(), err)
		},
	)

	go func() {
		err := srv.Serve(ln)
		if err != nil {
			log.Println("[redis-sim]", err)
		}
	}()
	return srv, tcpAddr.Port, nil
}

func startRedis(t testing.TB) func() {
	if !*redisSim {
		t.Log("[redis-sim] disabled")
		return func() {}
	}

	srv, redisPort, err := StartRedis(":")
	if err != nil {
		t.Error(err)
		return func() {}
	}
	flag.Set("redis.address", "127.0.0.1:"+strconv.Itoa(redisPort))
	flag.Set("redis.password", "")

	t.Log("[redis-sim] listen at:", *redisAddress)

	return func() {
		srv.Close()
	}
}

func clearRedis(t *testing.T, c *redis.Client, key string) {
	ctx := context.Background()
	err := c.Del(ctx, key).Err()
	if nil != err {
		t.Logf("DEL %s failed, %v", key, err)
	}
}

func redisTest(t *testing.T, cb func(client *redis_gateway, c *redis.Client)) {
	cancel := startRedis(t)
	defer cancel()

	redis_client, err := newRedis(*redisAddress, *redisPassword)
	if nil != err {
		t.Error(err)
		return
	}
	defer redis_client.Close()

	c := redis.NewClient(&redis.Options{
		Addr:         *redisAddress,
		Password:     *redisPassword,
		DialTimeout:  1 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})
	defer c.Close()

	for i := 0; i < 10; i++ {
		clearRedis(t, c, fmt.Sprintf("a%v", i))
	}

	cb(redis_client, c)
}

func checkResult(t *testing.T, c *redis.Client, key, excepted string) {
	ctx := context.Background()
	s, err := c.Get(ctx, key).Result()
	if nil != err {
		t.Errorf("GET %s failed, %v", key, err)
	} else if excepted != s {
		t.Errorf("check %s failed, actual is %v, excepted is %v", key, s, excepted)
	}
}

func TestRedis(t *testing.T) {
	redisTest(t, func(redis_client *redis_gateway, c *redis.Client) {
		redis_client.c <- &redis_request{commands: [][]string{{"SET", "a1", "1223"}}}
		redis_client.c <- &redis_request{commands: [][]string{{"SET", "a2", "1224"}}}
		redis_client.Send([][]string{{"SET", "a3", "1225"}})
		redis_client.Send([][]string{{"SET", "a4", "1226"}})
		redis_client.Send([][]string{{"SET", "a5", "1227"}})

		time.Sleep(2 * time.Second)

		checkResult(t, c, "a1", "1223")
		checkResult(t, c, "a2", "1224")
		checkResult(t, c, "a3", "1225")
		checkResult(t, c, "a4", "1226")
		checkResult(t, c, "a5", "1227")
	})
}

func TestRedisEmpty(t *testing.T) {
	redisTest(t, func(redis_client *redis_gateway, c *redis.Client) {
		redis_client.c <- &redis_request{commands: [][]string{}}
		redis_client.Send([][]string{{}})

		redis_client.c <- &redis_request{commands: [][]string{{"SET", "a1", "1223"}}}
		redis_client.c <- &redis_request{commands: [][]string{{"SET", "a2", "1224"}}}
		redis_client.Send([][]string{{"SET", "a3", "1225"}})
		redis_client.Send([][]string{{"SET", "a4", "1226"}})
		redis_client.Send([][]string{{"SET", "a5", "1227"}})

		time.Sleep(2 * time.Second)

		checkResult(t, c, "a1", "1223")
		checkResult(t, c, "a2", "1224")
		checkResult(t, c, "a3", "1225")
		checkResult(t, c, "a4", "1226")
		checkResult(t, c, "a5", "1227")
	})
}

func TestRedisConnectFailed(t *testing.T) {
	redis_client, err := newRedis("127.0.0.1:3", "")
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
