package ip_test

import (
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