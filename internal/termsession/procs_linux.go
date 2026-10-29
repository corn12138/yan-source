package termsession

import (
	"os"
	"strconv"
)

func all() []int {
	ents, _ := os.ReadDir("/proc")
	var out []int
	for _, e := range ents {
		if pid, err := strconv.Atoi(e.Name()); err == nil {
			out = append(out, pid)
		}
	}
	return out
}
