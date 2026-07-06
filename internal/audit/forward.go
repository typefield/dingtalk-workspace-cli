package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type HTTPForwarder struct {
	url    string
	token  string
	redact RedactLevel
	client *http.Client
}

func NewHTTPForwarder(url, token string, redact RedactLevel) *HTTPForwarder {
	return &HTTPForwarder{
		url:    url,
		token:  token,
		redact: redact,
		client: &http.Client{Timeout: 3 * time.Second},
	}
}

func (f *HTTPForwarder) Forward(evt Event) {
	go f.send(evt)
}

func (f *HTTPForwarder) send(evt Event) {
	var body []byte
	var err error
	if f.redact != RedactNone {
		body, err = RedactEventJSON(evt, f.redact)
	} else {
		body, err = json.Marshal(evt)
	}
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
