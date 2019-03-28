package main

import (
	"net"
	"reflect"
	"testing"
)

func init() {
	wgPath = "tests/wg"
	wgQuickPath = "tests/wg-quick"
}

func Test_wgKeyPair(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		want1   string
		wantErr bool
	}{
		// see tests/wg for values
		{"generate fixed values", "ILICZ3yBMCGAWNIq5Pn0bewBVimW3Q2yRVJ/Be+b1Uc=", "VceweY6x/QdGXEQ6frXrSd8CwUAInUmqIc6G/qi8FHo=", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := wgKeyPair()
			if (err != nil) != tt.wantErr {
				t.Errorf("wgKeyPair() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("wgKeyPair() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("wgKeyPair() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_wgState_assignOverlayAddr(t *testing.T) {
	type args struct {
		ipnet *net.IPNet
		name  string
	}
	tests := []struct {
		name string
		args args
		want net.IP
	}{
		{
			"assign in big ipv4 net",
			args{&net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)}, "test"},
			net.ParseIP("10.221.153.165"), // if we ever have to change this, we should probably also mark it as a breaking change
		},
		{
			"assign in ipv6 net",
			args{&net.IPNet{IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(32, 128)}, "test"},
			net.ParseIP("2001:db8:c575:7277:b806:e994:13dd:99a5"), // if we ever have to change this, we should probably also mark it as a breaking change
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wg := &wgState{}
			wg.assignOverlayAddr(tt.args.ipnet, tt.args.name)

			if !reflect.DeepEqual(wg.OverlayAddr, tt.want) {
				t.Errorf("assignOverlayAddr() set = %s, want %s", wg.OverlayAddr, tt.want)
			}
		})
	}
}

// This is just to ensure - if we ever change the hashing function - that it spreads the results in a way that at least
// avoids the most obvious collisions.
func Test_wgState_assignOverlayAddr_no_obvious_collisions(t *testing.T) {
	ipnet := &net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}
	assignments := make(map[string]string)
	for _, n := range []string{"test", "test1", "test2", "1test", "2test"} {
		wg := &wgState{}
		wg.assignOverlayAddr(ipnet, n)
		if assigned, ok := assignments[wg.OverlayAddr.String()]; ok {
			t.Errorf("IP assignment collision: hash(%s) = hash(%s)", n, assigned)
		}
		assignments[wg.OverlayAddr.String()] = n
	}
}
