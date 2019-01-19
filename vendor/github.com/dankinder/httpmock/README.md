# httpmock

<a href="https://godoc.org/github.com/dankinder/httpmock"><img src="https://godoc.org/github.com/dankinder/httpmock?status.svg" alt="GoDoc" /></a>
<a href="https://goreportcard.com/report/github.com/dankinder/httpmock"><img src="https://goreportcard.com/badge/github.com/dankinder/httpmock" alt="Go Report Card" /></a>
<a href="https://travis-ci.org/dankinder/httpmock"><img src="https://travis-ci.org/dankinder/httpmock.svg?branch=master" alt="Build Status" /></a>

This library builds on Go's built-in [httptest](https://golang.org/pkg/net/http/httptest/) library, adding a more
mockable interface that can be used easily with other mocking tools like
[testify/mock](https://godoc.org/github.com/stretchr/testify/mock). It does this by providing a Handler that receives
HTTP components separately rather than an `*http.Request` object.

Where the typical [http.Handler](https://golang.org/pkg/net/http/#Handler) interface is:
```go
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}
```
This library provides a server with the following interface, which works naturally with mocking libraries:
```go
// Handler is the interface used by httpmock instead of http.Handler so that it can be mocked very easily.
type Handler interface {
	Handle(method, path string, body []byte) Response
}
```

## Examples

The most primitive example, the `OKHandler`, just returns `200 OK` to everything.
```go
s := httpmock.NewServer(&httpmock.OKHandler{})
defer s.Close()

// Make any requests you want to s.URL(), using it as the mock downstream server
```

This example uses MockHandler, a Handler that is a [testify/mock](https://godoc.org/github.com/stretchr/testify/mock)
object.

```go
downstream := &httpmock.MockHandler{}

// A simple GET that returns some pre-canned content
downstream.On("Handle", "GET", "/object/12345", mock.Anything).Return(httpmock.Response{
    Body: []byte(`{"status": "ok"}`),
})

s := httpmock.NewServer(downstream)
defer s.Close()

//
// Make any requests you want to s.URL(), using it as the mock downstream server
//

downstream.AssertExpectations(t)
```

The httpmock package also provides helpers for checking calls using json objects, like so:

```go
// This tests a hypothetical "echo" endpoint, which returns the body we pass to it.
type Obj struct {
    A string `json:"a"`
}

o := &Obj{A: "aye"}

// JSONMatcher ensures that this mock is triggered only when the HTTP body, when deserialized, matches the given
// object. Here, this mock response will get triggered only if `{"a":"aye"}` is sent.
downstream.On("Handle", "POST", "/echo", httpmock.JSONMatcher(o)).Return(httpmock.Response{
    Body: httpmock.ToJSON(o),
})
```
