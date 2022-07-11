package isuhttp

import (
	"net"
	"net/http"
	"time"
)

const (
	maxIdleConns        = 1000
	maxIdleConnsPerHost = 1000
	keepAliveTime       = 90 * time.Second
)

var (
	newTransport *http.Transport
)

func init() {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return
	}
	newTransport = defaultTransport.Clone()

	newTransport.MaxIdleConns = maxIdleConns
	newTransport.MaxIdleConnsPerHost = maxIdleConnsPerHost
	newTransport.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: keepAliveTime,
	}).DialContext

	http.DefaultTransport = newTransport
}

func ClientSetting(client *http.Client) {
	if newTransport == nil {
		return
	}

	client.Transport = newTransport
}
