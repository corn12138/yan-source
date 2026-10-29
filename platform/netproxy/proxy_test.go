package netproxy

import (
	"net/http"
	"net/url"
	"testing"
)

func TestMacProxySettings(t *testing.T) {
	text := `<dictionary> {
 HTTPSEnable : 1
 HTTPSProxy : 127.0.0.1
 HTTPSPort : 7890
 ExceptionsList : <array> {
  0 : *.example.org
  1 : 10.0.0.0/8
  2 : <local>
 }
 __SCOPED__ : <dictionary> {
  en1 : <dictionary> {
   HTTPSEnable : 1
   HTTPSProxy : other-interface
   HTTPSPort : 9999
   ExceptionsList : <array> {
    0 : github.com
   }
  }
 }
}`
	st, err := parseMacSettings(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ raw, env, want string }{
		{"https://github.com/corn12138/yan-releases", "", "http://127.0.0.1:7890"},
		{"https://release-assets.githubusercontent.com/asset", "", "http://127.0.0.1:7890"},
		{"https://a.example.org/", "", ""}, {"https://10.1.2.3/", "", ""}, {"https://intranet/", "", ""},
		{"https://github.com/", "github.com", ""}, {"https://localhost/", "", ""},
	} {
		u, _ := url.Parse(tc.raw)
		p, err := st.proxy(u, tc.env)
		got := ""
		if p != nil {
			got = p.String()
		}
		if err != nil || got != tc.want {
			t.Fatalf("%s: %q %v", tc.raw, got, err)
		}
	}
	for _, tc := range []struct {
		input, want string
		invalid     bool
	}{
		{"HTTPSEnable : 0\nHTTPSProxy : disabled\nHTTPSPort : 123", "", false},
		{"HTTPSEnable : 1\nHTTPSProxy : ::1\nHTTPSPort : 8080", "http://[::1]:8080", false},
		{"SOCKSEnable : 1\nSOCKSProxy : localhost\nSOCKSPort : 1080", "socks5://localhost:1080", false},
		{"HTTPSEnable : 1\nHTTPSProxy : localhost\nHTTPSPort : 0", "", true},
		{"ProxyAutoConfigEnable : 1", "", true},
	} {
		st, err := parseMacSettings(tc.input)
		if (err != nil) != tc.invalid || !tc.invalid && st.config.HTTPSProxy != tc.want {
			t.Fatalf("%s: %+v %v", tc.input, st, err)
		}
	}
}

func TestEnvironmentProxyTakesPrecedence(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://configured-proxy:3128")
	t.Setenv("NO_PROXY", "excluded.example")
	t.Setenv("no_proxy", "")
	for _, host := range []string{"github.com", "excluded.example"} {
		req, _ := http.NewRequest("GET", "https://"+host+"/", nil)
		p, err := Proxy(req)
		if err != nil {
			t.Fatal(err)
		}
		if host == "github.com" && (p == nil || p.Host != "configured-proxy:3128") {
			t.Fatalf("proxy %v", p)
		}
		if host == "excluded.example" && p != nil {
			t.Fatalf("NO_PROXY ignored: %v", p)
		}
	}
}
