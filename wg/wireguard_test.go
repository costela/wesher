package wg

import (
	"net"
	"reflect"
	"testing"
)

func Test_wgState_assignOverlayAddr(t *testing.T) {
	type args struct {
		ipnet *net.IPNet
		name  string
	}
	_, ipv4net, _ := net.ParseCIDR("10.0.0.0/8")
	_, ipv6net, _ := net.ParseCIDR("2001:db8::/32")
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"assign in big ipv4 net",
			args{ipv4net, "test"},
			"10.221.153.165", // if we ever have to change this, we should probably also mark it as a breaking change
		},
		{
			"assign in ipv6 net",
			args{ipv6net, "test"},
			"2001:db8:c575:7277:b806:e994:13dd:99a5", // if we ever have to change this, we should probably also mark it as a breaking change
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wg := &WgState{}
			wg.AssignOverlayAddr(tt.args.ipnet, tt.args.name)

			if !reflect.DeepEqual(wg.OverlayAddr.IP.String(), tt.want) {
				t.Errorf("assignOverlayAddr() set = %s, want %s", wg.OverlayAddr, tt.want)
			}
		})
	}
}

// This is just to ensure - if we ever change the hashing function - that it spreads the results in a way that at least
// avoids the most obvious collisions.
func Test_wgState_assignOverlayAddr_no_obvious_collisions(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("10.0.0.0/24")
	assignments := make(map[string]string)
	for _, n := range []string{"test", "test1", "test2", "1test", "2test"} {
		wg := &WgState{}
		wg.AssignOverlayAddr(ipnet, n)
		if assigned, ok := assignments[wg.OverlayAddr.String()]; ok {
			t.Errorf("IP assignment collision: hash(%s) = hash(%s)", n, assigned)
		}
		assignments[wg.OverlayAddr.String()] = n
	}
}

// This should ensure the obvious fact that the same name should map to the same IP if called twice.
func Test_wgState_assignOverlayAddr_consistent(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("10.0.0.0/8")
	wg1 := &WgState{}
	wg1.AssignOverlayAddr(ipnet, "test")
	wg2 := &WgState{}
	wg2.AssignOverlayAddr(ipnet, "test")
	if wg1.OverlayAddr.String() != wg2.OverlayAddr.String() {
		t.Errorf("assignOverlayAddr() %s != %s", wg1.OverlayAddr, wg2.OverlayAddr)
	}
}

func Test_wgState_assignOverlayAddr_repeatable(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("10.0.0.0/8")
	wg := &WgState{}
	wg.AssignOverlayAddr(ipnet, "test")
	gen1 := wg.OverlayAddr.String()
	wg.AssignOverlayAddr(ipnet, "test")
	gen2 := wg.OverlayAddr.String()
	if gen1 != gen2 {
		t.Errorf("assignOverlayAddr() %s != %s", gen1, gen2)
	}
}
