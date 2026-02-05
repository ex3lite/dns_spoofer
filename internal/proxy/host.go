package proxy

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net/textproto"
	"strings"
)

var (
	ErrNoHostHeader = errors.New("no Host header found")
	ErrInvalidHTTP  = errors.New("invalid HTTP request")
)

// PeekHTTPHost reads HTTP headers from conn and extracts the Host header.
// Returns the host and a reader that replays the peeked bytes followed by the rest of conn.
func PeekHTTPHost(reader io.Reader) (string, io.Reader, error) {
	peekedBytes := new(bytes.Buffer)
	teeReader := io.TeeReader(reader, peekedBytes)
	bufReader := bufio.NewReader(teeReader)

	// Read the request line
	requestLine, err := bufReader.ReadString('\n')
	if err != nil {
		return "", nil, err
	}

	// Validate it looks like HTTP
	parts := strings.Fields(requestLine)
	if len(parts) < 3 || !strings.HasPrefix(parts[2], "HTTP/") {
		return "", nil, ErrInvalidHTTP
	}

	// Read headers using textproto
	tp := textproto.NewReader(bufReader)
	headers, err := tp.ReadMIMEHeader()
	if err != nil {
		return "", nil, err
	}

	host := headers.Get("Host")
	if host == "" {
		return "", nil, ErrNoHostHeader
	}

	// Remove port from host if present (for matching purposes)
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		// Check if it's not an IPv6 address
		if !strings.Contains(host, "[") {
			host = host[:colonIdx]
		}
	}

	return host, io.MultiReader(peekedBytes, reader), nil
}

// PeekHTTPHostSplice is similar to PeekHTTPHost but returns separate peeked buffer.
// This allows using splice(2) on Linux for better performance.
func PeekHTTPHostSplice(reader io.Reader) (string, *bytes.Buffer, error) {
	peekedBytes := new(bytes.Buffer)
	teeReader := io.TeeReader(reader, peekedBytes)
	bufReader := bufio.NewReader(teeReader)

	// Read the request line
	requestLine, err := bufReader.ReadString('\n')
	if err != nil {
		return "", nil, err
	}

	// Validate it looks like HTTP
	parts := strings.Fields(requestLine)
	if len(parts) < 3 || !strings.HasPrefix(parts[2], "HTTP/") {
		return "", nil, ErrInvalidHTTP
	}

	// Read headers using textproto
	tp := textproto.NewReader(bufReader)
	headers, err := tp.ReadMIMEHeader()
	if err != nil {
		return "", nil, err
	}

	host := headers.Get("Host")
	if host == "" {
		return "", nil, ErrNoHostHeader
	}

	// Remove port from host if present (for matching purposes)
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		// Check if it's not an IPv6 address
		if !strings.Contains(host, "[") {
			host = host[:colonIdx]
		}
	}

	return host, peekedBytes, nil
}
