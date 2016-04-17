package api

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockOracle struct{}

func (r *mockOracle) LookupAddr(string) ([]string, error)  { return []string{"localhost"}, nil }
func (r *mockOracle) LookupCountry(net.IP) (string, error) { return "Elbonia", nil }
func (r *mockOracle) LookupPort(net.IP, uint64) error      { return nil }
func (r *mockOracle) IsLookupAddrEnabled() bool            { return true }
func (r *mockOracle) IsLookupCountryEnabled() bool         { return true }
func (r *mockOracle) IsLookupPortEnabled() bool            { return true }

func newTestAPI() *API {
	return &API{
		oracle: &mockOracle{},
		ipFromRequest: func(*http.Request) (net.IP, error) {
			return net.ParseIP("127.0.0.1"), nil
		},
	}
}

func httpGet(url string, json bool, userAgent string) (string, int, error) {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, err
	}
	if json {
		r.Header.Set("Accept", "application/json")
	}
	r.Header.Set("User-Agent", userAgent)
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", 0, err
	}
	return string(data), res.StatusCode, nil
}

func TestClIHandlers(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	s := httptest.NewServer(newTestAPI().Handlers())

	var tests = []struct {
		url    string
		out    string
		status int
	}{
		{s.URL, "127.0.0.1\n", 200},
		{s.URL + "/ip", "127.0.0.1\n", 200},
		{s.URL + "/country", "Elbonia\n", 200},
		{s.URL + "/foo", "404 page not found", 404},
	}

	for _, tt := range tests {
		out, status, err := httpGet(tt.url /* json = */, false, "curl/7.2.6.0")
		if err != nil {
			t.Fatal(err)
		}
		if status != tt.status {
			t.Errorf("Expected %d, got %d", tt.status, status)
		}
		if out != tt.out {
			t.Errorf("Expected %q, got %q", tt.out, out)
		}
	}
}

func TestJSONHandlers(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	s := httptest.NewServer(newTestAPI().Handlers())

	var tests = []struct {
		url    string
		out    string
		status int
	}{
		{s.URL, `{"ip":"127.0.0.1","country":"Elbonia","hostname":"localhost"}`, 200},
		{s.URL + "/port/31337", `{"ip":"127.0.0.1","port":31337,"reachable":true}`, 200},
		{s.URL + "/foo", `{"error":"404 page not found"}`, 404},
	}

	for _, tt := range tests {
		out, status, err := httpGet(tt.url /* json = */, true, "curl/7.2.6.0")
		if err != nil {
			t.Fatal(err)
		}
		if status != tt.status {
			t.Errorf("Expected %d, got %d", tt.status, status)
		}
		if out != tt.out {
			t.Errorf("Expected %q, got %q", tt.out, out)
		}
	}
}

func TestIPFromRequest(t *testing.T) {
	var tests = []struct {
		in  *http.Request
		out net.IP
	}{
		{&http.Request{RemoteAddr: "1.3.3.7:9999"}, net.ParseIP("1.3.3.7")},
		{&http.Request{Header: http.Header{"X-Real-Ip": []string{"1.3.3.7"}}}, net.ParseIP("1.3.3.7")},
	}
	for _, tt := range tests {
		ip, err := ipFromRequest(tt.in)
		if err != nil {
			t.Fatal(err)
		}
		if !ip.Equal(tt.out) {
			t.Errorf("Expected %s, got %s", tt.out, ip)
		}
	}
}

func TestCLIMatcher(t *testing.T) {
	browserUserAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_4) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.28 " +
		"Safari/537.36"
	var tests = []struct {
		in  string
		out bool
	}{
		{"curl/7.26.0", true},
		{"Wget/1.13.4 (linux-gnu)", true},
		{"fetch libfetch/2.0", true},
		{"HTTPie/0.9.3", true},
		{"Go 1.1 package http", true},
		{"Go-http-client/1.1", true},
		{"Go-http-client/2.0", true},
		{browserUserAgent, false},
	}
	for _, tt := range tests {
		r := &http.Request{Header: http.Header{"User-Agent": []string{tt.in}}}
		if got := cliMatcher(r, nil); got != tt.out {
			t.Errorf("Expected %t, got %t for %q", tt.out, got, tt.in)
		}
	}
}
