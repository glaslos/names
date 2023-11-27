package names

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/phuslu/fastdns"
	"github.com/stretchr/testify/require"
)

var berr error

func BenchmarkMiekgParse(b *testing.B) {
	msg := new(dns.Msg)
	msg = msg.SetQuestion(dns.Fqdn("example.com."), dns.TypeNS)
	buf, err := msg.Pack()
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		berr = msg.Unpack(buf)
	}
	require.NoError(b, err)
}

func BenchmarkFastParse(b *testing.B) {
	msg := new(dns.Msg)
	msg = msg.SetQuestion(dns.Fqdn("example.com."), dns.TypeNS)
	buf, err := msg.Pack()
	require.NoError(b, err)
	fmsg := fastdns.Message{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		berr = fastdns.ParseMessage(&fmsg, buf, true)
	}
	require.NoError(b, err)
}
