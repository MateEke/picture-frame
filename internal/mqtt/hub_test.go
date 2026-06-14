package mqtt

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/testutil"
)

func TestHubAddOnConnectFanOut(t *testing.T) {
	hub := NewHub(testutil.NopLogger(), &fakeClient{})
	var a, b int32
	hub.AddOnConnect(func() { atomic.AddInt32(&a, 1) })
	hub.AddOnConnect(func() { atomic.AddInt32(&b, 1) })

	hub.onConnect()
	hub.onConnect()

	if atomic.LoadInt32(&a) != 2 || atomic.LoadInt32(&b) != 2 {
		t.Errorf("handlers should each fire on every connect: a=%d b=%d", a, b)
	}
}

func TestHubSubscribeAfterConnectIssuesImmediately(t *testing.T) {
	client := &fakeClient{}
	hub := NewHub(testutil.NopLogger(), client)

	hub.onConnect()
	hub.Subscribe("late/topic", 1, func([]byte) {})
	if client.subs["late/topic"] == nil {
		t.Fatal("subscription after connect should be issued to the broker synchronously")
	}
}

func TestHubSubscribeAfterConnectLogsButRecordsOnBrokerError(t *testing.T) {
	client := &fakeClient{}
	hub := NewHub(testutil.NopLogger(), client)
	hub.onConnect()

	client.subErr = errors.New("rejected")
	hub.Subscribe("retry/topic", 1, func([]byte) {})

	client.subErr = nil
	client.subs = nil
	hub.onConnect()
	if client.subs["retry/topic"] == nil {
		t.Error("recorded sub should be replayed on the next connect")
	}
}

func TestHubSubscribeReplayedOnEveryConnect(t *testing.T) {
	client := &fakeClient{}
	hub := NewHub(testutil.NopLogger(), client)

	var calls int32
	hub.Subscribe("topic/a", 1, func([]byte) { atomic.AddInt32(&calls, 1) })

	hub.onConnect()
	if client.subs["topic/a"] == nil {
		t.Fatal("subscription not replayed on first connect")
	}
	client.subs["topic/a"]([]byte("x"))

	client.subs = nil // simulate broker forgetting on reconnect
	hub.onConnect()
	if client.subs["topic/a"] == nil {
		t.Fatal("subscription not replayed on reconnect")
	}
	client.subs["topic/a"]([]byte("y"))

	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("handler call count = %d, want 2", calls)
	}
}

func TestHubPublishPassthrough(t *testing.T) {
	client := &fakeClient{}
	hub := NewHub(testutil.NopLogger(), client)
	if err := hub.Publish("t", 1, true, []byte("p")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got, _ := client.lastPayload("t"); got != "p" {
		t.Errorf("payload: %q", got)
	}
}

func TestHubPublishForwardsError(t *testing.T) {
	want := errors.New("broker down")
	hub := NewHub(testutil.NopLogger(), &fakeClient{pubErr: want})
	if err := hub.Publish("t", 0, false, nil); !errors.Is(err, want) {
		t.Errorf("Publish err = %v, want %v", err, want)
	}
}

func TestHubConnectRetriesUntilSuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := &fakeClient{connectFails: 2}
		hub := NewHub(testutil.NopLogger(), client)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan bool, 1)
		go func() { done <- hub.Connect(ctx) }()

		time.Sleep(2*connectRetry + time.Second)
		synctest.Wait()
		if got := <-done; !got {
			t.Error("Connect returned false despite eventual success")
		}
	})
}

func TestHubConnectCancellable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := &fakeClient{connectFails: 1_000_000}
		hub := NewHub(testutil.NopLogger(), client)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan bool, 1)
		go func() { done <- hub.Connect(ctx) }()

		time.Sleep(connectRetry / 2)
		cancel()
		synctest.Wait()
		if got := <-done; got {
			t.Error("Connect returned true despite cancellation")
		}
	})
}

func TestHubConnectionLostClearsConnectedFlag(t *testing.T) {
	client := &fakeClient{}
	hub := NewHub(testutil.NopLogger(), client)
	hub.onConnect() // up

	client.onConnectionLost(errors.New("broker reset"))

	client.subs = nil
	hub.Subscribe("offline/topic", 0, func([]byte) {})
	if client.subs != nil {
		t.Errorf("Subscribe should not call client.Subscribe while disconnected, got %v", client.subs)
	}

	hub.onConnect()
	if client.subs["offline/topic"] == nil {
		t.Error("sub registered while disconnected should be replayed on reconnect")
	}
}

func TestHubDisconnectForwards(t *testing.T) {
	client := &fakeClient{}
	hub := NewHub(testutil.NopLogger(), client)
	hub.Disconnect()
	if !client.disconnected {
		t.Error("Hub.Disconnect should forward to client.Disconnect")
	}
}
