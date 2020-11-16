package nats2http

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const RequestMethod = "RequestMethod"

type HttpServiceAdapter struct {
	HttpHandler http.Handler // The http handler to dispatch to
	BaseURL     string       // The base URL to associate all paths with `http://server:port`
	Logger      natsserver.Logger
}

// NatsHandler returns nats.MsgHandler that will dispatch to the HTTP handler
func (a *HttpServiceAdapter) NatsHandler() nats.MsgHandler {
	return func(m *nats.Msg) {
		a.handle(m)
	}
}

func (a *HttpServiceAdapter) handle(m *nats.Msg) {
	res := &natsResponseWriter{}
	res.m.Subject = m.Reply
	req, err := a.toRequest(m)
	if err != nil {
		if a.Logger != nil {
			a.Logger.Errorf("error converting nats msg [%s] to http request: %v", m.Subject, err)
		}
		res.m.Header = a.asHeader(err)
	} else {
		a.HttpHandler.ServeHTTP(res, req)
	}
	if res.m.Subject != "" {
		if a.Logger != nil {
			a.Logger.Debugf("responding to request [%s]: [%s]", m.Subject, m.Header.Get("Status"))
		}
		m.RespondMsg(&res.m)
	}

}

func (a *HttpServiceAdapter) asHeader(err error) http.Header {
	h := http.Header{}
	h.Add("Status", "400")
	h.Add("Description", err.Error())
	return h
}

func (a *HttpServiceAdapter) toRequest(m *nats.Msg) (*http.Request, error) {
	r := new(http.Request)
	p := strings.Replace(m.Subject, ".", "/", -1)
	reqURI := fmt.Sprintf("%s/%s", a.BaseURL, p)
	u, err := url.Parse(reqURI)
	if err != nil {
		return r, err
	}

	r.Method = "GET"
	if m.Header != nil {
		v := m.Header.Get(RequestMethod)
		if v != "" {
			r.Method = strings.ToUpper(v)
		}
	}
	r.URL = u
	r.Body = ioutil.NopCloser(bytes.NewReader(m.Data))
	r.RemoteAddr = "FROM_NATS"
	r.RequestURI = reqURI
	r.Header = m.Header
	r.Proto = "HTTP/1.0"
	r.ProtoMajor = 1
	r.ProtoMinor = 0
	return r, nil
}

type natsResponseWriter struct {
	m nats.Msg
}

func (r *natsResponseWriter) Header() http.Header {
	if r.m.Header == nil {
		r.m.Header = http.Header{}
	}
	return r.m.Header
}

func (r *natsResponseWriter) getStatus() string {
	status := ""
	if r.m.Header != nil {
		status = r.m.Header.Get("Status")
	}
	return status
}

func (r *natsResponseWriter) Write(data []byte) (int, error) {
	if r.getStatus() == "" {
		r.WriteHeader(http.StatusOK)
	}
	r.m.Data = append(r.m.Data, data...)
	return len(data), nil
}

func (r *natsResponseWriter) WriteHeader(code int) {
	if r.m.Header == nil {
		r.m.Header = http.Header{}
	}
	r.m.Header.Add("Status", strconv.Itoa(code))
	if code != http.StatusOK {
		description := http.StatusText(code)
		if description == "" {
			description = fmt.Sprintf("%d - Unknown Status", code)
		}
		r.m.Header.Add("Description", description)
	}
}
