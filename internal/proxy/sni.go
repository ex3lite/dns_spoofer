package proxy

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"time"
)

// readOnlyConn is a net.Conn wrapper that only supports Read operations.
// Used to extract SNI from TLS ClientHello without completing the handshake.
type readOnlyConn struct {
	reader io.Reader
}

func (c readOnlyConn) Read(p []byte) (int, error)         { return c.reader.Read(p) }
func (c readOnlyConn) Write(p []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (c readOnlyConn) Close() error                       { return nil }
func (c readOnlyConn) LocalAddr() net.Addr                { return nil }
func (c readOnlyConn) RemoteAddr() net.Addr               { return nil }
func (c readOnlyConn) SetDeadline(t time.Time) error      { return nil }
func (c readOnlyConn) SetReadDeadline(t time.Time) error  { return nil }
func (c readOnlyConn) SetWriteDeadline(t time.Time) error { return nil }

// readClientHello reads a TLS ClientHello from the reader and extracts the SNI.
// Uses crypto/tls.Server with GetConfigForClient callback to capture ClientHelloInfo.
func readClientHello(reader io.Reader) (*tls.ClientHelloInfo, error) {
	var hello *tls.ClientHelloInfo

	err := tls.Server(readOnlyConn{reader: reader}, &tls.Config{
		GetConfigForClient: func(argHello *tls.ClientHelloInfo) (*tls.Config, error) {
			hello = new(tls.ClientHelloInfo)
			*hello = *argHello
			return nil, nil
		},
	}).Handshake()

	// Handshake will always fail because readOnlyConn doesn't support Write.
	// We only care about the error if we didn't get the ClientHello.
	if hello == nil {
		return nil, err
	}

	return hello, nil
}

// PeekClientHello reads the TLS ClientHello from conn without consuming the bytes.
// Returns the ClientHelloInfo and a reader that replays the peeked bytes followed by the rest of conn.
func PeekClientHello(reader io.Reader) (*tls.ClientHelloInfo, io.Reader, error) {
	peekedBytes := new(bytes.Buffer)

	hello, err := readClientHello(io.TeeReader(reader, peekedBytes))
	if err != nil {
		return nil, nil, err
	}

	return hello, io.MultiReader(peekedBytes, reader), nil
}

// PeekClientHelloSplice is similar to PeekClientHello but returns separate peeked buffer.
// This allows using splice(2) on Linux for better performance.
func PeekClientHelloSplice(reader io.Reader) (*tls.ClientHelloInfo, *bytes.Buffer, error) {
	peekedBytes := new(bytes.Buffer)

	hello, err := readClientHello(io.TeeReader(reader, peekedBytes))
	if err != nil {
		return nil, nil, err
	}

	return hello, peekedBytes, nil
}
