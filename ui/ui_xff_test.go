package pitchforkui

/*
 * $ go test trident.li/pitchfork/ui -v -run UI_XFF
 * ok  	command-line-arguments	1.053s
 */

import (
	"errors"
	"net"
	"testing"
	pf "trident.li/pitchfork/lib"
)

func XFFC(trusted []string) (xffc []*net.IPNet) {
	for t := range trusted {
		x := trusted[t]
		_, xc, err := net.ParseCIDR(x)
		if err != nil {
			err = errors.New("Trusted XFF IP " + x + " is invalid: " + err.Error())
			return
		}

		/* Add it to the pre-parsed list */
		xffc = append(xffc, xc)
	}

	return
}

func NewCUI() PfUI {
	return NewPfUI(pf.NewPfCtx(nil, nil, nil, nil, nil), nil, nil, nil)
}

func ParseClientIP(tn string, t *testing.T, remaddr string, xff string, xffc []string, e_ip net.IP, e_addr string) {
	cui := NewCUI()

	ip, addr, err := cui.ParseClientIP(remaddr, xff, XFFC(xffc))

	if err != nil {
		t.Errorf("[%s] Error: %s", tn, err.Error())
	}

	if !ip.Equal(e_ip) {
		t.Errorf("[%s] Expected ip %q, got %q", tn, e_ip, ip)
	}

	if addr != e_addr {
		t.Errorf("[%s] Expected addr: %q, got %q", tn, e_addr, addr)
	}
}

func TestUI_ParseClientIP_XFF_Empty(t *testing.T) {
	remaddr := "127.0.0.1:12345"
	xff := ""
	xffc := []string{"127.0.0.1/8"}
	e_ip := net.ParseIP("127.0.0.1")
	e_addr := "127.0.0.1"

	ParseClientIP("XFF_Empty", t, remaddr, xff, xffc, e_ip, e_addr)
}

/* Untrusted remote, ok XFF */
func TestUI_ParseClientIP_XFF_Untrusted(t *testing.T) {
	remaddr := "192.0.2.1:12345"
	xff := "192.0.2.2"
	xffc := []string{"127.0.0.1/8"}
	e_ip := net.ParseIP("192.0.2.1")
	e_addr := "192.0.2.2, 192.0.2.1"

	ParseClientIP("XFF_Untrusted", t, remaddr, xff, xffc, e_ip, e_addr)
}

/* Trusted remote, ok XFF */
func TestUI_ParseClientIP_XFF_Trusted(t *testing.T) {
	remaddr := "127.0.0.1:12345"
	xff := "192.0.2.2"
	xffc := []string{"127.0.0.1/8"}
	e_ip := net.ParseIP("192.0.2.2")
	e_addr := "192.0.2.2, 127.0.0.1"

	ParseClientIP("XFF_Trusted", t, remaddr, xff, xffc, e_ip, e_addr)
}

/* Trusted remote, faked XFF header */
func TestUI_ParseClientIP_XFF_Faked(t *testing.T) {
	remaddr := "127.0.0.1:12345"
	xff := "127.0.0.1 192.0.2.2"
	xffc := []string{"127.0.0.1/8"}
	e_ip := net.ParseIP("192.0.2.2")
	e_addr := "127.0.0.1, 192.0.2.2, 127.0.0.1"

	ParseClientIP("XFF_Faked", t, remaddr, xff, xffc, e_ip, e_addr)
}
