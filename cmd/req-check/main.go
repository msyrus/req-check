package main

import (
	"encoding/json"
	"flag"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// MiB is 1 mega byte
const MiB = 1 << (10 * 2)

var (
	port, bcap int
)

func main() {
	flag.IntVar(&port, "port", 8080, "server port")
	flag.IntVar(&bcap, "cap", 512, "max request body bytes in response")
	flag.Parse()

	http.ListenAndServe(":"+strconv.Itoa(port), http.HandlerFunc(HandleReq))
}

// HandleReq serves req info
func HandleReq(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{}
	resp["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	resp["method"] = r.Method
	resp["uri"] = r.Host + r.RequestURI
	resp["query"] = r.URL.Query()
	resp["headers"] = r.Header
	resp["remote_addr"] = r.RemoteAddr

	defer r.Body.Close()
	body, cpd, err := parseBody(r)
	if err != nil {
		resp["body_error"] = err.Error()
	}
	resp["body_capped"] = cpd
	resp["body"] = body
	if body != nil {
		if data, ok := body.([]byte); ok {
			typ := http.DetectContentType(data)
			if strings.HasPrefix(typ, "text/") {
				resp["body"] = string(data)
			}
			resp["body_context_type"] = typ
		}
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(".json"))
	w.WriteHeader(200)

	json.NewEncoder(w).Encode(resp)
}

func parseBody(r *http.Request) (interface{}, bool, error) {
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, false, err
	}

	switch typ {
	case "application/json":
		body := map[string]interface{}{}
		if err = json.NewDecoder(r.Body).Decode(body); err != nil {
			return nil, false, err
		}

	case "application/x-www-form-urlencoded":
		if err = r.ParseForm(); err != nil {
			return nil, false, err
		}
		return r.PostForm, false, nil

	case "multipart/form-data":
		if err = r.ParseMultipartForm(2 * MiB); err != nil {
			return nil, false, err
		}
		return r.MultipartForm, false, nil
	}

	body := make([]byte, bcap+1)
	capped := false
	n, err := r.Body.Read(body)
	if err != nil && err != io.EOF {
		return nil, false, err
	}
	body = body[:n]

	if len(body) > bcap {
		capped = true
		body = body[:bcap]
	}

	return body, capped, nil
}
