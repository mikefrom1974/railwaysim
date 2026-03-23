package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	exchangeName = "train.commands"
	exchangeType = "topic"
)

type Rabbit struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// Connect does mTLS and sets up our train's queue (receive-only channel)
func (r *Rabbit) Connect(trainID int, mqURL, keyPEM, certPEM, caPEM string) (<-chan amqp.Delivery, error) {
	// load certs and ca
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("failure parsing train cert/key: %w", err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM([]byte(caPEM)); !ok {
		return nil, fmt.Errorf("failure parsing CA PEM into cert pool")
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
	}

	// connect to RabbitMQ
	rCfg := amqp.Config{
		TLSClientConfig: tlsConfig,
		// uncomment to override plan user:pw with cert name for external auth
		// SASL: []amqp.Authentication{&amqp.ExternalAuth{}},
	}
	r.conn, err = amqp.DialConfig(mqURL, rCfg)
	if err != nil {
		return nil, fmt.Errorf("failure connecting to rabbitMQ: %w", err)
	}
	r.channel, err = r.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failure opening mq channel: %w", err)
	}

	// declare exchange to make sure it exists
	err = r.channel.ExchangeDeclare(exchangeName, exchangeType, true, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failure declaring exchange: %w", err)
	}

	// declare / bind our train's queue
	qName := fmt.Sprintf("train.%d.cmds", trainID)
	bKey := fmt.Sprintf("train.%d.*", trainID)
	q, err := r.channel.QueueDeclare(qName, true, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failure declaring train queue: %w", err)
	}

	err = r.channel.QueueBind(q.Name, bKey, exchangeName, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failure binding queue: %w", err)
	}

	return r.channel.Consume(qName, "", true, false, false, false, nil)
}

func (r *Rabbit) Close() {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		r.conn.Close()
	}
}
