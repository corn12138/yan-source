//go:build !darwin && !linux

package termsession

// Windows 的 ConPTY 关掉时，系统结束挂在它上面的控制台程序；这里没有会话要收拾。

func all() []int  { return nil }
func sid(int) int { return -1 }
func kill(int)    {}
