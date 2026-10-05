package ip_test

import (
	"net/http/httptest"
	"testing"

	"github.com/Ahmed-Idani/ip"
)

func TestExtractIP(t *testing.T){
	testCases := []struct{
		name string
		remote_addr string
		expected string
		wantErr bool
	}{

		{
			name: "extract IPv4",
			remote_addr: "129.22.22.111:2222",
			expected: "129.22.22.111",
		},
		{
			name: "extract IPv6 --test 1",
			remote_addr: "[2001:db8:0:1::10]:3333",
			expected: "2001:db8:0:1::10",
		},
		{
			name: "extract IPv6 --test 2",
			remote_addr: "[fe80::1]:9",
			expected: "fe80::1",
		},
		{
			name: "empty address",
			remote_addr: "",
			wantErr: true,
		},
		{
			name: "garbage",
			remote_addr: "garbage",
			wantErr: true,
		},
		{
			name: "IPv4 without port",
			remote_addr: "1.2.3.4",
			wantErr: true,
		},

	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T){
			result, err := ip.ExtractIP(tc.remote_addr)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected an error, got: %s", result)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.String() != tc.expected {
				t.Errorf("expected: %s, got: %s", tc.expected, result)
			}

		})
	}
}

func TestClientIP(t *testing.T){
	testCases := []struct{
		name string
		remote_addr string
		real_ip string
		expected string
	}{
		{
			name: "direct visitor",
			remote_addr: "102.157.178.188:51234",
			expected: "102.157.178.188",
		},
		{
			name: "behind nginx",
			remote_addr: "127.0.0.1:40000",
			real_ip: "102.157.178.188",
			expected: "102.157.178.188",
		},
		{
			name: "behind nginx over ipv6 loopback",
			remote_addr: "[::1]:40000",
			real_ip: "2001:db8::7",
			expected: "2001:db8::7",
		},
		{
			name: "spoofed header from outside is ignored",
			remote_addr: "8.8.8.8:5555",
			real_ip: "102.157.178.188",
			expected: "8.8.8.8",
		},
		{
			name: "garbage header from nginx falls back",
			remote_addr: "127.0.0.1:40000",
			real_ip: "not-an-ip",
			expected: "127.0.0.1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T){
			r := httptest.NewRequest("GET", "/x", nil)
			r.RemoteAddr = tc.remote_addr
			if tc.real_ip != "" {
				r.Header.Set("X-Real-IP", tc.real_ip)
			}
			result, err := ip.ClientIP(r)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.String() != tc.expected {
				t.Errorf("expected: %s, got: %s", tc.expected, result)
			}
		})
	}
}