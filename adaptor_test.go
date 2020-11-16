package nats2http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	nsserver "github.com/nats-io/nats-server/server"
	nstest "github.com/nats-io/nats-server/test"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

type servers struct {
	ht      *httptest.Server
	ns      *nsserver.Server
	clients []*nats.Conn
}

func NewServers(h http.HandlerFunc) *servers {
	ht := httptest.NewServer(h)
	ns := nstest.RunRandClientPortServer()

	return &servers{ht: ht, ns: ns}
}

func (ts *servers) Stop() {
	for _, nc := range ts.clients {
		nc.Close()
	}
	ts.ht.Close()
	ts.ns.Shutdown()
}

func (ts *servers) client() (*nats.Conn, error) {
	nc, err := nats.Connect(ts.ns.ClientURL())
	if err != nil {
		return nil, err
	}
	ts.clients = append(ts.clients, nc)
	return nc, nil
}

func setRequestMethod(m *nats.Msg, method string) {
	m.Header = http.Header{}
	m.Header.Set(RequestMethod, strings.ToUpper(method))
}

func TestHttpMethod(t *testing.T) {
	ts := NewServers(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("%s %s", r.Method, r.URL.Path)))
	})
	defer ts.Stop()

	nc, err := ts.client()
	require.NoError(t, err)

	a := HttpServiceAdapter{BaseURL: "http://localhost", HttpHandler: ts.ht.Config.Handler}
	nc.Subscribe(">", a.NatsHandler())

	cn, err := ts.client()
	require.NoError(t, err)

	r := &nats.Msg{}
	r.Subject = "hello.world"
	setRequestMethod(r, "get")

	m, err := cn.RequestMsg(r, time.Second)
	require.NoError(t, err)
	require.Equal(t, "GET /hello/world", string(m.Data))

	setRequestMethod(r, "post")
	m, err = cn.RequestMsg(r, time.Second)
	require.NoError(t, err)
	require.Equal(t, "POST /hello/world", string(m.Data))

	setRequestMethod(r, "put")
	m, err = cn.RequestMsg(r, time.Second)
	require.NoError(t, err)
	require.Equal(t, "PUT /hello/world", string(m.Data))
}

func TestHttpError(t *testing.T) {
	ts := NewServers(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	defer ts.Stop()

	nc, err := ts.client()
	require.NoError(t, err)
	a := HttpServiceAdapter{BaseURL: "http://localhost", HttpHandler: ts.ht.Config.Handler}
	nc.Subscribe(">", a.NatsHandler())

	cn, err := ts.client()
	require.NoError(t, err)
	m, err := cn.Request("bad.request", nil, time.Second)
	require.NoError(t, err)
	require.NotNil(t, m.Header)
	require.Equal(t, "404", m.Header.Get("Status"))
	require.Equal(t, "Not Found", m.Header.Get("Description"))
}

func TestHttpQuery(t *testing.T) {
	ts := NewServers(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query()["name"]
		w.Write([]byte(fmt.Sprintf("Hello %s", name)))
	})
	defer ts.Stop()

	nc, err := ts.client()
	require.NoError(t, err)
	a := HttpServiceAdapter{BaseURL: "http://localhost", HttpHandler: ts.ht.Config.Handler}
	nc.Subscribe(">", a.NatsHandler())
	cn, err := ts.client()
	require.NoError(t, err)
	m, err := cn.Request("hello?name=Alberto", nil, time.Second)
	require.NoError(t, err)
	require.NotNil(t, m.Header)
	require.Equal(t, "200", m.Header.Get("Status"))
	require.Equal(t, []byte("Hello Alberto"), m.Data)
}
