package etchosts

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestEtcHosts_writeEntryWithBanner(t *testing.T) {
	type args struct {
		banner string
		ip     string
		names  []string
	}

	eh := &EtcHosts{}

	tests := []struct {
		name    string
		args    args
		wantTmp string
		wantErr bool
	}{
		{"do not write empty ip", args{DefaultBanner, "", []string{"somename", "someothername"}}, "", false},
		{"do not write empty names", args{DefaultBanner, "1.2.3.4", []string{}}, "", false},
		{"complete entry", args{DefaultBanner, "1.2.3.4", []string{"somename", "someothername"}}, fmt.Sprintf("1.2.3.4\tsomename someothername\t%s\n", DefaultBanner), false},
		{"custom banner", args{"# somebanner", "1.2.3.4", []string{"somename", "someothername"}}, fmt.Sprintf("1.2.3.4\tsomename someothername\t%s\n", "# somebanner"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := &bytes.Buffer{}
			if err := eh.writeEntryWithBanner(tmp, tt.args.banner, tt.args.ip, tt.args.names); (err != nil) != tt.wantErr {
				t.Errorf("writeEntryWithBanner() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotTmp := tmp.String(); gotTmp != tt.wantTmp {
				t.Errorf("writeEntryWithBanner() got:\n%#v, want\n%#v", gotTmp, tt.wantTmp)
			}
		})
	}
}

func TestEtcHosts_writeEntries(t *testing.T) {
	type fields struct {
		Banner string
		Path   string
		Logger logrus.StdLogger
	}
	type args struct {
		orig       io.Reader
		ipsToNames map[string][]string
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantDest string
		wantErr  bool
	}{
		{
			"simple empty write",
			fields{},
			args{strings.NewReader(""), map[string][]string{"1.2.3.4": {"foo", "bar"}}},
			"1.2.3.4\tfoo bar\t# ! MANAGED AUTOMATICALLY !\n",
			false,
		},
		{
			"do not touch comments",
			fields{},
			args{strings.NewReader("# some comment\n"), map[string][]string{"1.2.3.4": {"foo", "bar"}}},
			"# some comment\n1.2.3.4\tfoo bar\t# ! MANAGED AUTOMATICALLY !\n",
			false,
		},
		{
			"do not touch existing entries",
			fields{},
			args{strings.NewReader("4.3.2.1 hostname1 hostname2\n"), map[string][]string{"1.2.3.4": {"foo", "bar"}}},
			"4.3.2.1 hostname1 hostname2\n1.2.3.4\tfoo bar\t# ! MANAGED AUTOMATICALLY !\n",
			false,
		},
		{
			"remove managed entry not in map",
			fields{},
			args{strings.NewReader("4.3.2.1 fooz baarz # ! MANAGED AUTOMATICALLY !\n"), map[string][]string{"1.2.3.4": {"foo", "bar"}}},
			"1.2.3.4\tfoo bar\t# ! MANAGED AUTOMATICALLY !\n",
			false,
		},
		{
			"custom banner",
			fields{Banner: "# somebanner"},
			args{strings.NewReader(""), map[string][]string{"1.2.3.4": {"foo", "bar"}}},
			"1.2.3.4\tfoo bar\t# somebanner\n",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := &EtcHosts{
				Banner: tt.fields.Banner,
				Path:   tt.fields.Path,
				Logger: tt.fields.Logger,
			}
			dest := &bytes.Buffer{}
			if err := eh.writeEntries(tt.args.orig, dest, tt.args.ipsToNames); (err != nil) != tt.wantErr {
				t.Errorf("EtcHosts.writeEntries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotDest := dest.String(); gotDest != tt.wantDest {
				t.Errorf("EtcHosts.writeEntries() = '%#v', want '%#v'", gotDest, tt.wantDest)
			}
		})
	}
}
