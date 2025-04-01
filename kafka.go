package delayed_job

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

type kafkaHandler struct {
	addresses []string
	topic     string
	message   string
}

func newKafkaHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}

	addresses := stringsWithDefault(params, "addresses", ",", nil)
	if len(addresses) == 0 {
		return nil, errors.New("'addresses' is required.")
	}

	topic := stringWithDefault(params, "topic", "")
	if topic == "" {
		return nil, errors.New("'topic' is required")
	}

	content := stringWithDefault(params, "content", "")
	if 0 == len(content) {
		return nil, errors.New("'content' is required")
	}

	if args, ok := params["arguments"]; ok {
		args = preprocessArgs(args)
		if props, ok := args.(map[string]interface{}); ok {
			if _, ok := props["self"]; !ok {
				props["self"] = params
				defer delete(props, "self")
			}
		}
		var e error
		content, e = genText(content, args)
		if nil != e {
			return nil, e
		}
	}

	return &kafkaHandler{
		addresses: addresses,
		topic:     topic,
		message:   content,
	}, nil
}

func (self *kafkaHandler) Perform() error {
	// make a writer that produces to topic-A, using the least-bytes distribution
	w := &kafka.Writer{
		Addr:                   kafka.TCP(self.addresses...),
		Topic:                  self.topic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	messages := []kafka.Message{
		{
			Value: []byte(self.message),
		},
	}

	var err error
	const retries = 3
	for i := 0; ; i++ {
		if i > retries {
			return errors.New("failed to write messages:" + err.Error())
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// attempt to create topic prior to publishing the message
		err = w.WriteMessages(ctx, messages...)
		if errors.Is(err, kafka.LeaderNotAvailable) || errors.Is(err, context.DeadlineExceeded) {
			time.Sleep(time.Millisecond * 250)
			continue
		}
		if err == nil {
			break
		}
	}

	if err := w.Close(); err != nil {
		return errors.New("failed to close writer:" + err.Error())
	}
	return nil
}

func (self *kafkaHandler) send(to *net.UDPAddr) error {
	c, e := net.DialUDP("udp", nil, to)
	if nil != e {
		return e
	}
	defer c.Close()

	fmt.Println(c.RemoteAddr(), self.message)
	_, e = c.Write([]byte(self.message))
	return e
}

func init() {
	Handlers["kafka"] = newKafkaHandler
	Handlers["kafka_command"] = newKafkaHandler
}
