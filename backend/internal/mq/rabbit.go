// rabbit.go：RabbitMQ 工具层
package mq

import (
	"backend/internal/config"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

/*
  连接 RabbitMQ
  声明队列
  发送消息
  消费消息
  关闭连接
*/

type RabbitMQ struct {
	conn *amqp.Connection // conn 是进程内复用的 RabbitMQ TCP 连接。
}

// AMQPChannel 包装一条具体的 AMQP Channel。
// Channel 应该由单一并发角色创建、使用和关闭。
type AMQPChannel struct {
	ch *amqp.Channel
}

var (
	errRabbitMQConnectionUnavailable = errors.New("rabbitmq connection is unavailable")
	errAMQPChannelUnavailable        = errors.New("amqp channel is unavailable")
)

// NewRabbitMQ 只创建 Connection，不预先创建共享 Channel。
func NewRabbitMQ(cfg config.RabbitMQConfig) (*RabbitMQ, error) {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/", cfg.Username, cfg.Password, cfg.Host, cfg.Port)
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	return &RabbitMQ{conn: conn}, nil
}

// NewChannel 从复用的 Connection 上创建一条独立 Channel。
func (r *RabbitMQ) NewChannel() (*AMQPChannel, error) {
	if r == nil || r.conn == nil {
		return nil, errRabbitMQConnectionUnavailable
	}
	ch, err := r.conn.Channel()
	if err != nil {
		return nil, err
	}
	return &AMQPChannel{ch: ch}, nil
}

// Close 关闭进程级 Connection，应由创建 RabbitMQ 的 main 调用。
func (r *RabbitMQ) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}
	return r.conn.Close()
}

// Close 只关闭当前 Channel，不影响同一 Connection 上的其他 Channel。
func (c *AMQPChannel) Close() error {
	if c == nil || c.ch == nil {
		return nil
	}
	return c.ch.Close()
}

func (c *AMQPChannel) raw() (*amqp.Channel, error) {
	if c == nil || c.ch == nil {
		return nil, errAMQPChannelUnavailable
	}
	return c.ch, nil
}

/*
声明队列

	queueName：队列名
	true：持久化队列，RabbitMQ 重启后队列还在
	false：没有消费者时不自动删除
	false：不排他，允许多个连接访问
	false：不等待 RabbitMQ 响应，通常写 false
	nil：额外参数，暂时不用
*/
func (c *AMQPChannel) DeclareQueue(queueName string) error {
	ch, err := c.raw()
	if err != nil {
		return err
	}
	//告诉 RabbitMQ，准备一个队列用来放消息
	_, err = ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	return err
}

// 发送消息
func (c *AMQPChannel) Publish(ctx context.Context, queueName string, body string) error {
	return c.publish(ctx, queueName, "text/plain", []byte(body))
}

func (c *AMQPChannel) PublishJSONBody(ctx context.Context, queueName string, body string) error {
	return c.publish(ctx, queueName, "application/json", []byte(body))
}

func (c *AMQPChannel) PublishJSON(ctx context.Context, queueName string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.publish(ctx, queueName, "application/json", body)
}

func (c *AMQPChannel) publish(ctx context.Context, queueName, contentType string, body []byte) error {
	ch, err := c.raw()
	if err != nil {
		return err
	}
	return ch.PublishWithContext(
		ctx,
		"",        // exchange 传空字符串，使用 RabbitMQ 默认交换机。
		queueName, // routing key 与队列名一致。
		false,
		false,
		amqp.Publishing{
			ContentType:  contentType,
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// 消费消息
func (c *AMQPChannel) Consume(queueName string) (<-chan amqp.Delivery, error) {
	ch, err := c.raw()
	if err != nil {
		return nil, err
	}
	return ch.Consume(
		queueName,
		"",
		false, //autoAck 不需要自动确认消息
		/*
					不自动确认消息。
			  worker 处理完消息后，需要手动 d.Ack(false)。

			  为什么要这样？

			  如果自动确认，worker 刚拿到消息 RabbitMQ 就删除了。万一 worker 处理到一半崩
			  了，这条消息就丢了。

			  手动 Ack 更安全：

			  处理成功 -> d.Ack(false)
			  处理失败 -> d.Nack(false, true)
		*/
		false,
		false,
		false,
		nil,
	)
}
