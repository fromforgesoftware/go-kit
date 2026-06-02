package httpclient

import (
	"net/url"
	"strings"
)

func joinURL(base, rel string) (*url.URL, error) {
	b, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	r, err := url.Parse(rel)
	if err != nil {
		return nil, err
	}
	if r.IsAbs() {
		return r, nil
	}
	out := *b
	if r.Path != "" {
		if strings.HasSuffix(out.Path, "/") {
			out.Path += strings.TrimPrefix(r.Path, "/")
		} else if r.Path[0] == '/' {
			out.Path = r.Path
		} else {
			out.Path += "/" + r.Path
		}
	}
	if r.RawQuery != "" {
		if out.RawQuery == "" {
			out.RawQuery = r.RawQuery
		} else {
			out.RawQuery += "&" + r.RawQuery
		}
	}
	if r.Fragment != "" {
		out.Fragment = r.Fragment
	}
	return &out, nil
}
