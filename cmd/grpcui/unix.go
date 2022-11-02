//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package main

var (
	unix = flags.Bool("unix", false,
		`Indicates that the server address is the path to a Unix domain socket.`)
)

func init() {
	isUnixSocket = func() bool {
		return *unix
	}
}
