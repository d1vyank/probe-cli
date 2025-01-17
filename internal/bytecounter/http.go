package bytecounter

import (
	"io"
	"net/http"

	"github.com/ooni/probe-cli/v3/internal/model"
)

// MaybeWrapHTTPTransport takes in input an HTTPTransport and either wraps it
// to perform byte counting, if this counter is not nil, or just returns to the
// caller the original transport, when the counter is nil.
func (c *Counter) MaybeWrapHTTPTransport(txp model.HTTPTransport) model.HTTPTransport {
	if c != nil {
		txp = WrapHTTPTransport(txp, c)
	}
	return txp
}

// httpTransport is a model.HTTPTransport that counts bytes.
type httpTransport struct {
	HTTPTransport model.HTTPTransport
	Counter       *Counter
}

// WrapHTTPTransport creates a new byte-counting-aware HTTP transport.
func WrapHTTPTransport(txp model.HTTPTransport, counter *Counter) model.HTTPTransport {
	return &httpTransport{
		HTTPTransport: txp,
		Counter:       counter,
	}
}

var _ model.HTTPTransport = &httpTransport{}

// CloseIdleConnections implements model.HTTPTransport.CloseIdleConnections.
func (txp *httpTransport) CloseIdleConnections() {
	txp.HTTPTransport.CloseIdleConnections()
}

// RoundTrip implements model.HTTPTRansport.RoundTrip
func (txp *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		req.Body = &httpBodyWrapper{
			account: txp.Counter.CountBytesSent,
			rc:      req.Body,
		}
	}
	txp.estimateRequestMetadata(req)
	resp, err := txp.HTTPTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	txp.estimateResponseMetadata(resp)
	resp.Body = &httpBodyWrapper{
		account: txp.Counter.CountBytesReceived,
		rc:      resp.Body,
	}
	return resp, nil
}

// Network implements model.HTTPTransport.Network.
func (txp *httpTransport) Network() string {
	return txp.HTTPTransport.Network()
}

func (txp *httpTransport) estimateRequestMetadata(req *http.Request) {
	txp.Counter.CountBytesSent(len(req.Method))
	txp.Counter.CountBytesSent(len(req.URL.String()))
	for key, values := range req.Header {
		for _, value := range values {
			txp.Counter.CountBytesSent(len(key))
			txp.Counter.CountBytesSent(len(": "))
			txp.Counter.CountBytesSent(len(value))
			txp.Counter.CountBytesSent(len("\r\n"))
		}
	}
	txp.Counter.CountBytesSent(len("\r\n"))
}

func (txp *httpTransport) estimateResponseMetadata(resp *http.Response) {
	txp.Counter.CountBytesReceived(len(resp.Status))
	for key, values := range resp.Header {
		for _, value := range values {
			txp.Counter.CountBytesReceived(len(key))
			txp.Counter.CountBytesReceived(len(": "))
			txp.Counter.CountBytesReceived(len(value))
			txp.Counter.CountBytesReceived(len("\r\n"))
		}
	}
	txp.Counter.CountBytesReceived(len("\r\n"))
}

type httpBodyWrapper struct {
	account func(int)
	rc      io.ReadCloser
}

var _ io.ReadCloser = &httpBodyWrapper{}

func (r *httpBodyWrapper) Read(p []byte) (int, error) {
	count, err := r.rc.Read(p)
	if count > 0 {
		r.account(count)
	}
	return count, err
}

func (r *httpBodyWrapper) Close() error {
	return r.rc.Close()
}
