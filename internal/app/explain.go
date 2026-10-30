package app

import (
	"errors"
	"net"

	"github.com/renyijiu/tingyu/internal/player"
)

// Explain words err for the window. A network that cannot be reached says
// what to check, a player.Notice is shown as it is, and anything else says
// only what failed. Addresses and the system's own wording are nothing a
// listener can act on, so they go to the log instead.
func Explain(err error, failed string) string {
	var notice player.Notice
	var netErr net.Error
	var opErr *net.OpError
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &notice):
		return string(notice)
	case errors.As(err, &netErr) && netErr.Timeout():
		return "连接超时，请检查网络后重试"
	case errors.As(err, &opErr), errors.As(err, &dnsErr):
		return "网络连接失败，请检查网络后重试"
	}
	return failed
}
