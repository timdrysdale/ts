package ts

import (
	"bytes"
	"context"
	"sync"
)

type Config struct {
	Destination   string
	MaxFrameBytes int
}

type Client struct {
	Cancel        context.CancelFunc
	Context       context.Context
	Destination   string
	MaxFrameBytes int
	Receive       chan []byte
	Send          chan []byte
}

type mutexBuffer struct {
	mux sync.Mutex
	b   bytes.Buffer
}
