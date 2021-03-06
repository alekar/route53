package route53

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type request struct {
	method string
	path   string
	params *url.Values
	body   interface{}
}

func (r *request) url() *url.URL {
	url, _ := url.Parse("https://route53.amazonaws.com")
	url.Path = r.path

	// Most requests don't have params.
	if r.params != nil {
		url.RawQuery = r.params.Encode()
	}

	return url
}

type errorResponse struct {
	Type      string `xml:"Error>Type"`
	Code      string `xml:"Error>Code"`
	Message   string `xml:"Error>Message"`
	RequestID string `xml:"RequestId"`
}

func (r53 *Route53) run(req request, res interface{}) error {
	return r53.doRun(req, res, 0)
}

func (r53 *Route53) doRun(req request, res interface{}, try int) error {
	hreq := &http.Request{
		Method:     req.method,
		URL:        req.url(),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
	}
	r53.sign(hreq)

	if debug {
		fmt.Fprintf(os.Stderr, "-- request\n%+v\n\n", hreq)
	}

	if req.body != nil {
		data, err := xml.Marshal(req.body)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "-- error marshalling\n%s\n%+v\n", err, req.body)
			}
			return err
		}

		if debug {
			ppData, _ := xml.MarshalIndent(req.body, " ", "    ")
			ppBody_s11n := strings.Replace(string(ppData), "<AliasTarget></AliasTarget>", "- <AliasTarget></AliasTarget>", -1)
			if !r53.IncludeWeight {
				ppBody_s11n = strings.Replace(string(ppData), "<Weight>0</Weight>", "- <Weight>0</Weight>", -1)
			}
			fmt.Fprintf(os.Stderr, "-- body\n%s\n\n", xml.Header+ppBody_s11n)
		}

		body_s11n := strings.Replace(string(data), "<AliasTarget></AliasTarget>", "", -1)
		if !r53.IncludeWeight {
			body_s11n = strings.Replace(string(data), "<Weight>0</Weight>", "", -1)
		}
		hreq.Body = ioutil.NopCloser(bytes.NewBufferString(xml.Header + body_s11n))
	}

	hres, err := http.DefaultClient.Do(hreq)
	if err != nil {
		return err
	}
	defer hres.Body.Close()

	if debug {
		fmt.Fprintf(os.Stderr, "-- response\n%+v\n\n", hres)
	}

	body, err := ioutil.ReadAll(hres.Body)
	if err != nil {
		return err
	}

	bodyReadCloser := ioutil.NopCloser(bytes.NewReader(body))

	if hres.StatusCode == 403 && try == 0 {
		if debug {
			fmt.Fprintln(os.Stderr, "-- forbidden. updating auth and retrying")
		}
		// if 403 it probably means our auth is outdated. Lets update it and retry. (only retry once)
		r53.updateAuth() // this causes all other requests to wait because of the authLock. no big deal though.
		r53.doRun(req, res, try+1)
	} else if hres.StatusCode != 200 {
		eres := errorResponse{}

		err := xml.NewDecoder(bodyReadCloser).Decode(&eres)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "-- error unmarshalling\n%s\n%s\n\n", err, string(body))
			}
			return fmt.Errorf("could not parse: %s", string(body))
		} else {
			if debug {
				ppBody, _ := xml.MarshalIndent(eres, " ", "    ")
				fmt.Fprintf(os.Stderr, "-- body\n%s\n\n", string(ppBody))
			}
			return fmt.Errorf("%s: %s", eres.Code, eres.Message)
		}
	}

	err = xml.NewDecoder(bodyReadCloser).Decode(res)

	if debug {
		if err != nil {
			// Decode error, cannot pretty print this response.
			fmt.Fprintf(os.Stderr, "-- error unmarshalling\n%s\n%s\n\n", err, string(body))
		} else {
			ppBody, _ := xml.MarshalIndent(res, " ", "    ")
			fmt.Fprintf(os.Stderr, "-- body\n%s\n\n", string(ppBody))
		}
	}

	return err
}
