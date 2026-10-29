package netproxy

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func parseMacSettings(text string) (settings, error) {
	values := map[string]string{}
	var exceptions []string
	inExceptions := false
	skipDepth := 0
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		// Interface-scoped dictionaries must not replace the effective default route.
		if skipDepth > 0 {
			if strings.HasSuffix(line, "{") {
				skipDepth++
			} else if line == "}" {
				skipDepth--
			}
			continue
		}
		if line == "ExceptionsList : <array> {" {
			inExceptions = true
			continue
		}
		if strings.HasSuffix(line, "{") && line != "<dictionary> {" {
			skipDepth = 1
			continue
		}
		if line == "}" {
			inExceptions = false
			continue
		}
		key, value, ok := strings.Cut(line, " : ")
		if !ok {
			continue
		}
		if inExceptions {
			exceptions = append(exceptions, value)
		} else {
			values[key] = value
		}
	}
	var st settings
	st.excludeSimple = values["ExcludeSimpleHostnames"] == "1"
	for _, ex := range exceptions {
		if ex == "<local>" {
			st.excludeSimple = true
			continue
		}
		st.config.NoProxy += ex + ","
	}
	address := func(kind, scheme string) (string, error) {
		if values[kind+"Enable"] != "1" {
			return "", nil
		}
		host := values[kind+"Proxy"]
		port, err := strconv.Atoi(values[kind+"Port"])
		if err != nil || port < 1 || port > 65535 || host == "" || strings.ContainsAny(host, "/@ \t\r\n") {
			return "", fmt.Errorf("invalid %s system proxy", kind)
		}
		return (&url.URL{Scheme: scheme, Host: net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port))}).String(), nil
	}
	var err error
	if st.config.HTTPProxy, err = address("HTTP", "http"); err != nil {
		return st, err
	}
	if st.config.HTTPSProxy, err = address("HTTPS", "http"); err != nil {
		return st, err
	}
	socks, err := address("SOCKS", "socks5")
	if err != nil {
		return st, err
	}
	if st.config.HTTPProxy == "" {
		st.config.HTTPProxy = socks
	}
	if st.config.HTTPSProxy == "" {
		st.config.HTTPSProxy = socks
	}
	if st.config.HTTPProxy == "" && st.config.HTTPSProxy == "" && (values["ProxyAutoConfigEnable"] == "1" || values["ProxyAutoDiscoveryEnable"] == "1") {
		return st, fmt.Errorf("automatic proxy scripts are not supported for updates; configure an HTTP or HTTPS proxy")
	}
	return st, nil
}
