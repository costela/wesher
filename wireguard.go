package main

import (
	"crypto/md5"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

const wgConfPath = "/etc/wireguard/%s.conf"
const wgConfTpl = `
# this file was generated automatically by wesher - DO NOT MODIFY
[Interface]
PrivateKey = {{ .PrivKey }}
Address = {{ .OverlayAddr }}
ListenPort = {{ .Port }}

{{ range .Nodes }}
[Peer]
PublicKey = {{ .PubKey }}
Endpoint = {{ .Addr }}:{{ $.Port }}
AllowedIPs = {{ .OverlayAddr }}/32
{{ end }}`

type wgState struct {
	iface       string
	OverlayAddr net.IP
	Port        int
	PrivKey     string
	PubKey      string
}

func newWGConfig(iface string, port int) (*wgState, error) {
	if err := exec.Command("wg").Run(); err != nil {
		return nil, fmt.Errorf("could not exec wireguard: %s", err)
	}

	privKey, pubKey, err := wgKeyPair()
	if err != nil {
		return nil, err
	}

	wgState := wgState{
		iface:   iface,
		Port:    port,
		PrivKey: privKey,
		PubKey:  pubKey,
	}
	return &wgState, nil
}

func (wg *wgState) assignOverlayAddr(ipnet *net.IPNet, name string) {
	// TODO: this is way too brittle and opaque
	ip := []byte(ipnet.IP)
	bits, size := ipnet.Mask.Size()

	h := md5.New()
	h.Write([]byte(name))
	hb := h.Sum(nil)

	for i := 0; i < (size-bits)/8; i++ {
		ip[size/8-i-1] = hb[i]
	}
	wg.OverlayAddr = net.IP(ip)
}

func (wg *wgState) writeConf(nodes []node) error {
	tpl := template.Must(template.New("wgconf").Parse(wgConfTpl))
	out, err := os.OpenFile(
		fmt.Sprintf(wgConfPath, wg.iface),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0600,
	)
	if err != nil {
		return err
	}
	return tpl.Execute(out, struct {
		*wgState
		Nodes []node
	}{wg, nodes})
}

func (wg *wgState) downInterface() error {
	return exec.Command("wg-quick", "down", wg.iface).Run()
}

func (wg *wgState) upInterface() error {
	return exec.Command("wg-quick", "up", wg.iface).Run()
}

func wgKeyPair() (string, string, error) {
	cmd := exec.Command("wg", "genkey")
	outPriv := strings.Builder{}
	cmd.Stdout = &outPriv
	if err := cmd.Run(); err != nil {
		return "", "", err
	}

	cmd = exec.Command("wg", "pubkey")
	outPub := strings.Builder{}
	cmd.Stdout = &outPub
	cmd.Stdin = strings.NewReader(outPriv.String())
	if err := cmd.Run(); err != nil {
		return "", "", err
	}

	return strings.TrimSpace(outPriv.String()), strings.TrimSpace(outPub.String()), nil
}
