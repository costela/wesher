package wg

import (
	"net"
	"reflect"
	"testing"
)

func Test_State_AssignOverlayAddr(t *testing.T) {
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
			s := &State{}
			s.assignOverlayAddr(tt.args.ipnet, tt.args.name)

			if !reflect.DeepEqual(s.OverlayAddr.IP.String(), tt.want) {
				t.Errorf("assignOverlayAddr() set = %s, want %s", s.OverlayAddr, tt.want)
			}
		})
	}
}

// This is just to ensure - if we ever change the hashing function - that it spreads the results in a way that at least
// avoids the most obvious collisions.
func Test_State_AssignOverlayAddr_no_obvious_collisions(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("10.0.0.0/24")
	assignments := make(map[string]string)
	for _, n := range []string{"test", "test1", "test2", "1test", "2test"} {
		s := &State{}
		s.assignOverlayAddr(ipnet, n)
		if assigned, ok := assignments[s.OverlayAddr.String()]; ok {
			t.Errorf("IP assignment collision: hash(%s) = hash(%s)", n, assigned)
		}
		assignments[s.OverlayAddr.String()] = n
	}
}

// This should ensure the obvious fact that the same name should map to the same IP if called twice.
func Test_State_AssignOverlayAddr_consistent(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("10.0.0.0/8")
	s1 := &State{}
	s1.assignOverlayAddr(ipnet, "test")
	s2 := &State{}
	s2.assignOverlayAddr(ipnet, "test")
	if s1.OverlayAddr.String() != s2.OverlayAddr.String() {
		t.Errorf("assignOverlayAddr() %s != %s", s1.OverlayAddr, s2.OverlayAddr)
	}
}

func Test_State_AssignOverlayAddr_repeatable(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("10.0.0.0/8")
	s := &State{}
	s.assignOverlayAddr(ipnet, "test")
	gen1 := s.OverlayAddr.String()
	s.assignOverlayAddr(ipnet, "test")
	gen2 := s.OverlayAddr.String()
	if gen1 != gen2 {
		t.Errorf("assignOverlayAddr() %s != %s", gen1, gen2)
	}
}
