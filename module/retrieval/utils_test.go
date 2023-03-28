package retrieval

import (
	"net/url"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
)

func TestPeerIDFromMultiAddr(t *testing.T) {
	t.Run("valid multiaddr", func(t *testing.T) {
		ma := "/ip4/127.0.0.1/tcp/8080/p2p/QmZVcmaGp93b7VrfgyXhtPRbpzLyka1vTyuLkVQx3qN3hN"
		expectedID, _ := peer.Decode("QmZVcmaGp93b7VrfgyXhtPRbpzLyka1vTyuLkVQx3qN3hN")
		pid, err := peerIDFromMultiAddr(ma)
		assert.Nil(t, err)
		assert.Equal(t, expectedID, pid)
	})

	t.Run("invalid multiaddr", func(t *testing.T) {
		ma := "/ip4/127.0.0.1/tcp/8080/p2p/invalidID"
		_, err := peerIDFromMultiAddr(ma)
		assert.NotNil(t, err)
	})
}

func TestRoundtrip(t *testing.T) {
	samples := []string{
		"http://www.google.com/path/to/rsrc",
		"https://protocol.ai",
		"http://192.168.0.1:8080/admin",
		"https://[2a00:1450:400e:80d::200e]:443/",
		"https://[2a00:1450:400e:80d::200e]/",
	}

	for _, s := range samples {
		u, _ := url.Parse(s)
		mu, err := ToMultiaddr(u)
		if err != nil {
			t.Fatal(err)
		}
		u2, err := ToURL(mu)
		if err != nil {
			t.Fatal(err)
		}
		if u2.Scheme != u.Scheme {
			t.Fatalf("scheme didn't roundtrip. got %s expected %s", u2.Scheme, u.Scheme)
		}
		if u2.Host != u.Host {
			t.Fatalf("host didn't roundtrip. got %s, expected %s", u2.Host, u.Host)
		}
		if u2.Path != u.Path {
			t.Fatalf("path didn't roundtrip. got %s, expected %s", u2.Path, u.Path)
		}
	}
}

func TestTLSProtos(t *testing.T) {
	samples := []string{
		"/ip4/192.169.0.1/tls/http",
		"/ip4/192.169.0.1/https",
		"/ip4/192.169.0.1/http",
		"/dns4/protocol.ai/tls/ws",
		"/dns4/protocol.ai/wss",
		"/dns4/protocol.ai/ws",
	}

	expect := []string{
		"https://192.169.0.1",
		"https://192.169.0.1",
		"http://192.169.0.1",
		"wss://protocol.ai",
		"wss://protocol.ai",
		"ws://protocol.ai",
	}

	for i := range samples {
		m, err := multiaddr.NewMultiaddr(samples[i])
		if err != nil {
			t.Fatal(err)
		}

		u, err := ToURL(m)
		if err != nil {
			t.Fatal(err)
		}
		if u.String() != expect[i] {
			t.Fatalf("expected %s to convert to url %s, got %s", m.String(), expect[i], u.String())
		}
	}
}

func TestMultiaddrToNative(t *testing.T) {
	t.Run("http multiaddr", func(t *testing.T) {
		ma, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/8080/http")
		actual := multiaddrToNative("http", ma)
		assert.Equal(t, "http://127.0.0.1:8080", actual)
	})

	t.Run("https multiaddr", func(t *testing.T) {
		ma, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/8080/https")
		actual := multiaddrToNative("https", ma)
		assert.Equal(t, "https://127.0.0.1:8080", actual)
	})

	t.Run("unknown protocol", func(t *testing.T) {
		ma, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/8080/quic")
		actual := multiaddrToNative("quic", ma)
		assert.Equal(t, "/ip4/127.0.0.1/tcp/8080/quic", actual)
	})
}
