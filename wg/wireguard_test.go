package wg

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_State_AssignOverlayAddr(t *testing.T) {
	type args struct {
		prefix   netip.Prefix
		hostname string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"assign in big ipv4 net",
			args{netip.MustParsePrefix("10.0.0.0/8"), "test"},
			"10.221.153.165", // if we ever have to change this, we should probably also mark it as a breaking change
		},
		{
			"assign in small ipv4 net",
			args{netip.MustParsePrefix("10.0.0.0/24"), "test"},
			"10.0.0.165", // if we ever have to change this, we should probably also mark it as a breaking change
		},
		{
			"assign in ipv6 net",
			args{netip.MustParsePrefix("2001:db8::/32"), "test"},
			"2001:db8:c575:7277:b806:e994:13dd:99a5", // if we ever have to change this, we should probably also mark it as a breaking change
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &State{}
			err := s.assignOverlayAddr(tt.args.prefix, tt.args.hostname)
			require.NoError(t, err)

			assert.Equal(t, tt.want, s.OverlayAddr.String())
		})
	}
}

// This is just to ensure - if we ever change the hashing function - that it spreads the results in a way that at least
// avoids the most obvious collisions.
func Test_State_AssignOverlayAddr_no_obvious_collisions(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	assignments := make(map[string]string)
	for _, n := range []string{"test", "test1", "test2", "1test", "2test"} {
		s := &State{}
		err := s.assignOverlayAddr(prefix, n)
		require.NoError(t, err)

		assert.NotContainsf(t, assignments, s.OverlayAddr.String(), "IP assignment collision for hostname %q", n)

		assignments[s.OverlayAddr.String()] = n
	}
}

// This should ensure the obvious fact that the same name should map to the same IP if called twice.
func Test_State_AssignOverlayAddr_consistent(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	s1 := &State{}
	err := s1.assignOverlayAddr(prefix, "test")
	require.NoError(t, err)

	s2 := &State{}
	s2.assignOverlayAddr(prefix, "test")
	require.NoError(t, err)

	assert.Equal(t, s1.OverlayAddr.String(), s2.OverlayAddr.String())
}

func Test_State_AssignOverlayAddr_repeatable(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	s := &State{}
	err := s.assignOverlayAddr(prefix, "test")
	require.NoError(t, err)
	gen1 := s.OverlayAddr.String()

	err = s.assignOverlayAddr(prefix, "test")
	require.NoError(t, err)
	gen2 := s.OverlayAddr.String()

	assert.Equal(t, gen1, gen2)
}
