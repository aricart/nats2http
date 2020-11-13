package nats2http

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nats-io/nats.go"
)

const RequestMethod = "RequestMethod"

func Handle(m *nats.Msg, server http.Handler) {
	res := &natsResponseWriter{}
	res.m.Subject = m.Reply
	req, err := toRequest(m)
	if err != nil {
		res.m.Header = asHeader(err)
	} else {
		server.ServeHTTP(res, req)
	}
	m.RespondMsg(&res.m)
}

func asHeader(err error) http.Header {
	h := http.Header{}
	h.Add("Status", "400")
	h.Add("Description", err.Error())
	return h
}

func toRequest(m *nats.Msg) (*http.Request, error) {
	r := new(http.Request)
	p := strings.Replace(m.Subject, ".", "/", -1)
	reqURI := fmt.Sprintf("http://localhost:8080/%s", p)
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

func (r *natsResponseWriter) Write(data []byte) (int, error) {
	r.m.Data = append(r.m.Data, data...)
	return len(data), nil
}

func (r *natsResponseWriter) WriteHeader(statusCode int) {
	if r.m.Header == nil {
		r.m.Header = http.Header{}
	}
	r.m.Header.Add("Status", strconv.Itoa(statusCode))
}
