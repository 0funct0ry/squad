package hooks

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// netDisabledMessage is the exact error a hook sees when it touches the http
// module without --allow-net (PROMPTS.md M10c). A stub table is registered so
// this message surfaces instead of gopher-lua's generic
// "attempt to index a nil value".
const netDisabledMessage = "network access disabled — restart with --allow-net to enable the http module"

// httpMethods is the fixed allowlist; no arbitrary method strings.
var httpMethods = []string{"get", "post", "put", "patch", "delete"}

const (
	httpTimeout     = 5 * time.Second
	httpMaxBodySize = 1 << 20 // 1 MiB response cap
)

func registerHTTPModule(L *lua.LState, cfg SandboxConfig) {
	mod := L.NewTable()

	if !cfg.AllowNet {
		stub := L.NewFunction(func(l *lua.LState) int {
			l.RaiseError("%s", netDisabledMessage)
			return 0
		})
		for _, m := range httpMethods {
			L.SetField(mod, m, stub)
		}
		// Any other field (http.request, http.head, ...) raises the same
		// message rather than reading as nil.
		mt := L.NewTable()
		L.SetField(mt, "__index", stub)
		L.SetMetatable(mod, mt)
		L.SetGlobal("http", mod)
		return
	}

	client := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 && req.URL.Scheme != via[0].URL.Scheme {
				return http.ErrUseLastResponse
			}
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	for _, m := range httpMethods {
		method := strings.ToUpper(m)
		L.SetField(mod, m, L.NewFunction(func(l *lua.LState) int {
			return doHTTP(l, client, method)
		}))
	}
	L.SetGlobal("http", mod)
}

// doHTTP implements http.get(url, opts) / http.post(url, body, opts) and the
// put/patch/delete variants with the same shape as post.
func doHTTP(l *lua.LState, client *http.Client, method string) int {
	rawURL := l.CheckString(1)
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		l.RaiseError("http.%s: url must be an absolute http(s) URL", strings.ToLower(method))
		return 0
	}

	var body io.Reader
	optsIndex := 2
	if method != "GET" {
		if s, ok := l.Get(2).(lua.LString); ok {
			body = strings.NewReader(string(s))
		}
		optsIndex = 3
	}

	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		l.RaiseError("http.%s: %v", strings.ToLower(method), err)
		return 0
	}
	if opts, ok := l.Get(optsIndex).(*lua.LTable); ok {
		if headers, ok := opts.RawGetString("headers").(*lua.LTable); ok {
			headers.ForEach(func(k, v lua.LValue) {
				req.Header.Set(k.String(), v.String())
			})
		}
	}
	if req.Header.Get("Content-Type") == "" && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		l.RaiseError("http.%s: %v", strings.ToLower(method), err)
		return 0
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, httpMaxBodySize))
	if err != nil {
		l.RaiseError("http.%s: %v", strings.ToLower(method), err)
		return 0
	}

	out := l.NewTable()
	out.RawSetString("status", lua.LNumber(resp.StatusCode))
	out.RawSetString("body", lua.LString(string(data)))
	hdr := l.NewTable()
	for k := range resp.Header {
		hdr.RawSetString(k, lua.LString(resp.Header.Get(k)))
	}
	out.RawSetString("headers", hdr)
	l.Push(out)
	return 1
}
