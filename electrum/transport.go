package electrum

import (
	"bufio"
	"context"
	"crypto/tls"
	"log"
	"net"
	"time"
)

// TCPTransport store information about the TCP transport.
type TCPTransport struct {
	conn      net.Conn
	responses chan []byte
	errors    chan error
}

// DialerOption is a function that configures a TCPTransport.
type DialerOption func(*net.Dialer)

func WithTimeout(timeout time.Duration) DialerOption {
	return func(d *net.Dialer) {
		d.Timeout = timeout
	}
}

// NewTCPTransport opens a new TCP connection to the remote server.
func NewTCPTransport(
	ctx context.Context,
	addr string,
	options ...DialerOption,
) (*TCPTransport, error) {
	var d = net.Dialer{}

	for _, option := range options {
		option(&d)
	}

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	tcp := &TCPTransport{
		conn:      conn,
		responses: make(chan []byte),
		errors:    make(chan error),
	}

	go tcp.listen()

	return tcp, nil
}

// NewSSLTransport opens a new SSL connection to the remote server.
func NewSSLTransport(
	ctx context.Context,
	addr string,
	config *tls.Config,
	options ...DialerOption,
) (*TCPTransport, error) {
	dialer := tls.Dialer{
		NetDialer: &net.Dialer{},
		Config:    config,
	}
	for _, option := range options {
		option(dialer.NetDialer)
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	tcp := &TCPTransport{
		conn:      conn,
		responses: make(chan []byte),
		errors:    make(chan error),
	}

	go tcp.listen()

	return tcp, nil
}

func (t *TCPTransport) listen() {
	defer t.conn.Close()
	reader := bufio.NewReader(t.conn)

	for {
		line, err := reader.ReadBytes(nl)
		if err != nil {
			t.errors <- err
			break
		}
		if DebugMode {
			log.Printf(
				"%s [debug] %s -> %s",
				time.Now().Format("2006-01-02 15:04:05"),
				t.conn.RemoteAddr(),
				line,
			)
		}

		t.responses <- line
	}
}

// SendMessage sends a message to the remote server through the TCP transport.
func (t *TCPTransport) SendMessage(body []byte) error {
	if DebugMode {
		log.Printf(
			"%s [debug] %s <- %s",
			time.Now().Format("2006-01-02 15:04:05"),
			t.conn.RemoteAddr(),
			body,
		)
	}

	_, err := t.conn.Write(body)
	return err
}

// Responses returns chan to TCP transport responses.
func (t *TCPTransport) Responses() <-chan []byte {
	return t.responses
}

// Errors returns chan to TCP transport errors.
func (t *TCPTransport) Errors() <-chan error {
	return t.errors
}

func (t *TCPTransport) Close() error {
	return t.conn.Close()
}
