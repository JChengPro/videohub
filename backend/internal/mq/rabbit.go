// rabbit.go：RabbitMQ 工具层
package mq

import (
	"backend/internal/config"
	"context"
	"encoding/json"
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
	conn *amqp.Connection //conn RabbitMQ 的 TCP 连接
	ch   *amqp.Channel    //ch：通信通道，真正声明队列、发消息、收消息都通过 ch 做
}

// 构造函数
//  1. 连接 RabbitMQ
//  2. 创建 channel
//  3. 返回 RabbitMQ 对象给外部使用
func NewRabbitMQ(cfg config.RabbitMQConfig) (*RabbitMQ, error) {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/", cfg.Username, cfg.Password, cfg.Host, cfg.Port)
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &RabbitMQ{
		conn: conn,
		ch:   ch,
	}, nil
}

// 关闭函数
func (r *RabbitMQ) Close() {
	if r.ch != nil {
		r.ch.Close()
	}
	if r.conn != nil {
		r.conn.Close()
	}
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
func (r *RabbitMQ) DeclareQueue(queueName string) error {
	//告诉 RabbitMQ，准备一个队列用来放消息
	_, err := r.ch.QueueDeclare(
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
func (r *RabbitMQ) Publish(ctx context.Context, queueName string, body string) error {
	return r.ch.PublishWithContext(
		ctx,
		"",        //exchange 传 ""，不用交换机
		queueName, //routingKey 传 queueName
		false,
		false,
		amqp.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp.Persistent,
			Body:         []byte(body),
		},
	)
}

func (r *RabbitMQ) PublishJSONBody(ctx context.Context, queueName string, body string) error {
	return r.ch.PublishWithContext(
		ctx,
		"",
		queueName,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         []byte(body),
		},
	)
}

func (r *RabbitMQ) PublishJSON(ctx context.Context, queueName string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return r.ch.PublishWithContext(
		ctx,
		"",
		queueName,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// 消费消息
func (r *RabbitMQ) Consume(queueName string) (<-chan amqp.Delivery, error) {
	return r.ch.Consume(
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
