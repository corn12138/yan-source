package keymap

import "bytes"

// 读 VS Code 带注释的 JSON，移植自 Mayoi v0.7.0（MIT，internal/keymap/keymap.go 的 StripJSONC），没有改动。

// StripJSONC drops the comments of JSON, and the commas before a closing
// bracket or brace.
func StripJSONC(data []byte) []byte {
	return dropTrailingCommas(dropComments(data))
}

func dropComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case c == '"':
			j := skipString(data, i)
			out = append(out, data[i:j]...)
			i = j - 1
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			end := bytes.Index(data[i+2:], []byte("*/"))
			if end < 0 {
				return out
			}
			i += 2 + end + 1
		default:
			out = append(out, c)
		}
	}
	return out
}

func dropTrailingCommas(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch c {
		case '"':
			j := skipString(data, i)
			out = append(out, data[i:j]...)
			i = j - 1
			continue
		case ',':
			rest := bytes.TrimLeft(data[i+1:], " \t\r\n")
			if len(rest) > 0 && (rest[0] == ']' || rest[0] == '}') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// skipString returns the end of the string that starts at i, after its
// closing quote.
func skipString(data []byte, i int) int {
	j := i + 1
	for j < len(data) && data[j] != '"' {
		if data[j] == '\\' {
			j++
		}
		j++
	}
	return min(j+1, len(data))
}
