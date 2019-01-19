package httpmock

// OKHandler is a simple Handler that returns 200 OK responses for any request.
type OKHandler struct {
}

// Handle makes this implement the Handler interface.
func (r *OKHandler) Handle(method, path string, body []byte) Response {
	return Response{Status: 200}
}
