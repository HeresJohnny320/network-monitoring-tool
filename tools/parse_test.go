package tools

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestParsePingOutput(t *testing.T) {
	cases := []struct {
		name             string
		out              string
		sent, received   int
		avg, jitter, los float64
	}{
		{
			name: "linux",
			out: `PING 1.1.1.1 (1.1.1.1) 56(84) bytes of data.
64 bytes from 1.1.1.1: icmp_seq=1 ttl=57 time=10.0 ms
64 bytes from 1.1.1.1: icmp_seq=2 ttl=57 time=12.0 ms
64 bytes from 1.1.1.1: icmp_seq=4 ttl=57 time=14.0 ms

--- 1.1.1.1 ping statistics ---
4 packets transmitted, 3 received, 25% packet loss, time 3004ms
rtt min/avg/max/mdev = 10.0/12.0/14.0/1.6 ms`,
			sent: 4, received: 3, avg: 12, jitter: 2, los: 25,
		},
		{
			name: "macos/freebsd",
			out: `PING google.com (142.250.72.14): 56 data bytes
64 bytes from 142.250.72.14: icmp_seq=0 ttl=117 time=20.123 ms
64 bytes from 142.250.72.14: icmp_seq=1 ttl=117 time=22.123 ms`,
			sent: 2, received: 2, avg: 21.123, jitter: 2, los: 0,
		},
		{
			name: "windows",
			out:  "Pinging 1.1.1.1 with 32 bytes of data:\r\nReply from 1.1.1.1: bytes=32 time=9ms TTL=57\r\nReply from 1.1.1.1: bytes=32 time<1ms TTL=57\r\nRequest timed out.\r\nReply from 192.168.1.1: Destination host unreachable.\r\n",
			sent: 4, received: 2, avg: 5, jitter: 8, los: 50,
		},
		{
			name: "windows german",
			out:  "Antwort von 1.1.1.1: Bytes=32 Zeit=15ms TTL=57\r\n",
			sent: 1, received: 1, avg: 15, los: 0,
		},
		{
			name: "unreachable",
			out:  "ping: cannot resolve nowhere.invalid: Unknown host",
			sent: 4, received: 0, avg: -1, los: 100,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := ParsePingOutput("h", c.sent, c.out)
			if r.Received != c.received || !near(r.AvgMs, c.avg) || !near(r.JitterMs, c.jitter) || !near(r.LossPct, c.los) {
				t.Fatalf("got received=%d avg=%v jitter=%v loss=%v", r.Received, r.AvgMs, r.JitterMs, r.LossPct)
			}
		})
	}
}

func TestParseTracerouteOutput(t *testing.T) {
	unix := `traceroute to google.com (142.250.72.14), 30 hops max, 60 byte packets
 1  192.168.1.1  0.512 ms  0.456 ms  0.401 ms
 2  * * *
 3  10.0.0.1  5.123 ms 10.0.0.2  5.3 ms *
 4  2001:4860::1  9 ms  9.5 ms  10 ms`
	hops := ParseTracerouteOutput(unix)
	if len(hops) != 4 {
		t.Fatalf("want 4 hops, got %d", len(hops))
	}
	if hops[0].IP != "192.168.1.1" || *hops[0].Times[0] != 0.512 {
		t.Errorf("hop 1: %+v", hops[0])
	}
	if hops[1].IP != "*" || hops[1].Times[0] != nil || len(hops[1].Times) != 3 {
		t.Errorf("hop 2: %+v", hops[1])
	}
	if hops[2].IP != "10.0.0.1" || *hops[2].Times[1] != 5.3 || hops[2].Times[2] != nil {
		t.Errorf("hop 3: %+v", hops[2])
	}
	if hops[3].IP != "2001:4860::1" || *hops[3].Times[0] != 9 {
		t.Errorf("hop 4: %+v", hops[3])
	}

	windows := "\r\nTracing route to 1.1.1.1 over a maximum of 30 hops\r\n\r\n" +
		"  1    <1 ms    <1 ms    <1 ms  192.168.1.1\r\n" +
		"  2     *        *        *     Request timed out.\r\n" +
		"  3    12 ms    11 ms    13 ms  1.1.1.1\r\n\r\nTrace complete.\r\n"
	hops = ParseTracerouteOutput(windows)
	if len(hops) != 3 {
		t.Fatalf("want 3 hops, got %d", len(hops))
	}
	if hops[0].IP != "192.168.1.1" || *hops[0].Times[2] != 1 {
		t.Errorf("win hop 1: %+v", hops[0])
	}
	if hops[1].IP != "*" || hops[1].Times[0] != nil {
		t.Errorf("win hop 2: %+v", hops[1])
	}
	if hops[2].Hop != 3 || *hops[2].Times[1] != 11 {
		t.Errorf("win hop 3: %+v", hops[2])
	}
}

func TestParseOoklaOutput(t *testing.T) {
	out := []byte(`{"type":"log","timestamp":"2026-01-01T00:00:00Z","message":"Configuration - Couldn't connect","level":"warning"}
{"type":"result","timestamp":"2026-01-01T00:00:10Z","ping":{"jitter":0.5,"latency":8.25},"download":{"bandwidth":12500000,"bytes":1},"upload":{"bandwidth":2500000,"bytes":1},"packetLoss":0,"isp":"Example ISP","server":{"id":1234,"host":"speed.example.com","port":8080,"name":"Example","location":"Springfield","country":"United States"},"result":{"id":"abc","url":"https://www.speedtest.net/result/c/ABC"}}`)
	r, err := ParseOoklaOutput(out)
	if err != nil {
		t.Fatal(err)
	}
	if toMbps(r.Download) != 100 || toMbps(r.Upload) != 20 || r.Ping != 8.25 || r.ServerID != "1234" ||
		r.ISP != "Example ISP" || r.ResultURL != "https://www.speedtest.net/result/c/ABC" || r.ServerLocation != "Springfield, United States" {
		t.Fatalf("unexpected result %+v", r)
	}
	if r.PacketLoss == nil || *r.PacketLoss != 0 || r.Jitter == nil || *r.Jitter != 0.5 {
		t.Fatalf("jitter/loss not parsed: %+v", r)
	}
}

func TestParsePythonSpeedtestOutput(t *testing.T) {
	out := []byte(`{"download": 100000000.0, "upload": 20000000.0, "ping": 12.5, "server": {"id": "4321", "host": "st.example.net:8080", "name": "Shelbyville", "country": "United States", "sponsor": "X"}, "share": null, "client": {"isp": "Example ISP"}}`)
	r, err := ParsePythonSpeedtestOutput(out)
	if err != nil {
		t.Fatal(err)
	}
	if toMbps(r.Download) != 100 || toMbps(r.Upload) != 20 || r.ServerID != "4321" || r.ISP != "Example ISP" {
		t.Fatalf("unexpected result %+v", r)
	}
}
