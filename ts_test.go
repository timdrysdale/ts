package ts

import (
	"bytes"
	"testing"
	"time"

	"github.com/timdrysdale/tc"
)

func TestDial(t *testing.T) {

	config := &Config{
		MaxFrameBytes: 512,
		Destination:   "127.0.0.1:8877",
	}

	closed := make(chan struct{})

	c := New(config)
	go c.Run(closed)

	go func() { //echo server!
	LOOP:
		for {
			select {
			case <-closed:
				break LOOP
			case msg, ok := <-c.Receive:
				if ok {
					c.Send <- msg
				}
			}
		}

	}()

	time.Sleep(time.Millisecond)

	tcConfig := &tc.Config{
		MaxFrameBytes: 512,
		Destination:   "127.0.0.1:8877",
	}

	client := tc.New(tcConfig)

	go client.Run(closed)

	time.Sleep(time.Millisecond)

	greet := []byte("Greetings")
	client.Send <- greet

	select {
	case <-time.After(10 * time.Millisecond):
		t.Errorf("Receive timeout")
	case msg, ok := <-client.Receive:
		if ok {
			if bytes.Compare(msg, greet) != 0 {
				t.Errorf("Wrong message.\nWant: %s\nGot : %s\n", greet, msg)
			}
		}
	}

	close(closed)

}
