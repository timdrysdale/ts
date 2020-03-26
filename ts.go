package ts

import (
	"bufio"
	"io"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

func New(config *Config) *Client {

	maxFrameBytes := 1024000

	if config.MaxFrameBytes != 0 {
		maxFrameBytes = config.MaxFrameBytes
	}

	c := &Client{
		MaxFrameBytes: maxFrameBytes,
		Destination:   config.Destination,
		Send:          make(chan []byte),
		Receive:       make(chan []byte),
	}

	return c

}

func (c *Client) Run(closed chan struct{}) {

	addr, err := net.ResolveTCPAddr("tcp", c.Destination)
	if err != nil {
		log.Fatal("Could not resolve TCP addr")
	}

	ln, err := net.ListenTCP("tcp", addr)

	if err != nil {
		log.Errorf("Listener error: %v", err)
		return
	}
LOOP:
	for {
		select {
		case <-closed:
			break LOOP
		default:
			if err := ln.SetDeadline(time.Now().Add(time.Second)); err != nil {
				log.Fatal("Can't set deadline")
			}
			conn, err := ln.Accept()
			if err != nil {
				if opError, ok := err.(*net.OpError); ok {
					if !opError.Timeout() {
						log.WithField("Error", err).Error("Problem accepting connection")
					} //else it's a timeout and we don't care as prob no one connecting just now
				}
			} else {
				go c.handleConnection(conn, closed)
			}
		}
	}
}

func (c *Client) handleConnection(conn net.Conn, closed chan struct{}) {

	var frameBuffer mutexBuffer

	rawFrame := make([]byte, c.MaxFrameBytes)

	glob := make([]byte, c.MaxFrameBytes)

	frameBuffer.b.Reset() //else we send whole buffer on first flush

	reader := bufio.NewReader(conn)

	tCh := make(chan int)

	// write messages to the destination
	go func() {
		for {
			select {
			case data := <-c.Send:
				conn.Write(data)
			case <-closed:
				//put this option here to avoid spinning our wheels
			}
		}
	}()

	// Read from the buffer, blocking if empty
	go func() {

		for {

			tCh <- 0 //tell the monitoring routine we're alive

			n, err := io.ReadAtLeast(reader, glob, 1)
			log.WithField("Count", n).Debug("Read from buffer")
			if err == nil {

				frameBuffer.mux.Lock()

				_, err = frameBuffer.b.Write(glob[:n])

				frameBuffer.mux.Unlock()

				if err != nil {
					log.Errorf("%v", err) //was Fatal?
					return
				}

			} else {

				return // avoid spinning our wheels

			}
		}
	}()

	for {

		select {

		case <-tCh:

			// do nothing, just received data from buffer

		case <-time.After(1 * time.Millisecond):
			// no new data for >= 1mS weakly implies frame has been fully sent to us
			// this is two orders of magnitude more delay than when reading from
			// non-empty buffer so _should_ be ok, but recheck if errors crop up on
			// lower powered system.

			//flush buffer to internal send channel
			frameBuffer.mux.Lock()

			n, err := frameBuffer.b.Read(rawFrame)

			frame := rawFrame[:n]

			frameBuffer.b.Reset()

			frameBuffer.mux.Unlock()

			if err == nil && n > 0 {
				c.Receive <- frame
			}

		case <-closed:
			log.WithFields(log.Fields{"Destination": c.Destination}).Debug("tcp client closed")
			return
		}
	}
}
