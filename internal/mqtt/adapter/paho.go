// Package adapter implements mqtt.Client over eclipse/paho.mqtt.golang.
package adapter

import (
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	connectTimeout = 10 * time.Second
	opTimeout      = 5 * time.Second
	disconnectMs   = 250
	// willOffline is the Last Will payload, matching the Publisher's offline value.
	willOffline = "offline"
)

// Config describes the broker connection. WillTopic receives the Last Will.
type Config struct {
	Broker    string
	ClientID  string
	Username  string
	Password  string
	WillTopic string
}

// Paho is a paho-backed mqtt.Client with auto-reconnect.
type Paho struct {
	client           paho.Client
	onConnect        func()
	onConnectionLost func(err error)
}

// New builds a Paho client with a Last Will on cfg.WillTopic.
func New(cfg Config) *Paho {
	p := &Paho{}
	opts := paho.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID(cfg.ClientID)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}
	opts.SetAutoReconnect(true)
	// Bound each attempt so Connect() resolves before the Publisher retries.
	opts.SetConnectTimeout(connectTimeout)
	// Empty WillTopic → skip; strict brokers reject empty-topic Will in CONNECT.
	if cfg.WillTopic != "" {
		opts.SetWill(cfg.WillTopic, willOffline, 1, true)
	}
	opts.SetOnConnectHandler(func(paho.Client) {
		if p.onConnect != nil {
			p.onConnect()
		}
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		if p.onConnectionLost != nil {
			p.onConnectionLost(err)
		}
	})
	p.client = paho.NewClient(opts)
	return p
}

// OnConnect registers the handler invoked on every successful (re)connect.
func (p *Paho) OnConnect(f func()) { p.onConnect = f }

// OnConnectionLost registers the handler invoked when the connection drops.
func (p *Paho) OnConnectionLost(f func(err error)) { p.onConnectionLost = f }

// Connect makes one attempt, blocking until it resolves (bounded by
// SetConnectTimeout) so no attempt is left in flight when the caller retries.
func (p *Paho) Connect() error {
	t := p.client.Connect()
	t.Wait()
	return t.Error()
}

// Disconnect closes the connection, allowing in-flight work a short grace.
func (p *Paho) Disconnect() { p.client.Disconnect(disconnectMs) }

// Publish sends payload to topic and waits for the broker handshake (qos>0).
func (p *Paho) Publish(topic string, qos byte, retain bool, payload []byte) error {
	t := p.client.Publish(topic, qos, retain, payload)
	if !t.WaitTimeout(opTimeout) {
		return fmt.Errorf("mqtt publish to %s timed out", topic)
	}
	return t.Error()
}

// Subscribe registers handler for topic, passing through only the payload.
func (p *Paho) Subscribe(topic string, qos byte, handler func(payload []byte)) error {
	t := p.client.Subscribe(topic, qos, func(_ paho.Client, m paho.Message) {
		handler(m.Payload())
	})
	if !t.WaitTimeout(opTimeout) {
		return fmt.Errorf("mqtt subscribe to %s timed out", topic)
	}
	return t.Error()
}
