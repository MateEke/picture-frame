package startup_test

import (
	"testing"

	"github.com/MateEke/picture-frame/internal/startup"
)

func TestMakeRestartFuncEnqueues(t *testing.T) {
	ch := make(chan struct{}, 1)
	if err := startup.MakeRestartFunc(ch)(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case <-ch:
	default:
		t.Fatal("nothing enqueued")
	}
}

func TestMakeRestartFuncDropsWhenAlreadyQueued(t *testing.T) {
	ch := make(chan struct{}, 1)
	restart := startup.MakeRestartFunc(ch)

	if err := restart(); err != nil { // fills the buffer
		t.Fatalf("first call: %v", err)
	}
	if err := restart(); err != nil { // must drop, not block
		t.Fatalf("second call: %v", err)
	}

	<-ch // drain the single queued restart
	select {
	case <-ch:
		t.Fatal("expected the second request to be dropped")
	default:
	}
}
