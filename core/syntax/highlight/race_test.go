//go:build race

package highlight

// copies is how many times the tests repeat a sample. Under the race
// detector regexp2 runs some 20 times slower, and Chroma gives every lexer
// regexp a 250 ms match timeout (chroma/v2 regexp.go), so scanning an
// unclosed comment over 30 copies times out and lexes differently from the
// run it is compared with. Fewer copies keep each match inside the timeout;
// the edits and the comparisons are the same.
const copies = 6
