//go:build !windows

package clipboard

func ttyPath() string { return "/dev/tty" }
