package services

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/config"
)

func fakeSMTP(t *testing.T) (host string, port string, got *[]string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		fmt.Fprint(conn, "220 fake ESMTP\r\n")
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					inData = false
					fmt.Fprint(conn, "250 OK\r\n")
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				fmt.Fprint(conn, "250-fake\r\n250 OK\r\n")
			case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
				lines = append(lines, line)
				fmt.Fprint(conn, "250 OK\r\n")
			case strings.HasPrefix(line, "DATA"):
				inData = true
				fmt.Fprint(conn, "354 send it\r\n")
			case strings.HasPrefix(line, "QUIT"):
				fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				fmt.Fprint(conn, "250 OK\r\n")
			}
		}
	}()
	t.Cleanup(func() { ln.Close(); <-done })
	h, p, _ := net.SplitHostPort(ln.Addr().String())
	return h, p, &lines
}

func TestSendEmailUsesBareAddressesInTheEnvelope(t *testing.T) {
	host, port, captured := fakeSMTP(t)

	prev := config.Config
	config.Init(&config.Cfg{
		SMTPEnabled: "true", SMTPHost: host, SMTPPort: port,
		SMTPFrom: "Traceway Alerts <alerts@example.com>",
	})
	t.Cleanup(func() { config.Init(prev) })

	err := SendEmail(context.Background(), Email{
		To:       []string{"Ops Team <ops@example.com>", "  padded@example.com  ", "plain@example.com"},
		Subject:  "Test",
		Template: "invitation",
		Title:    "Test",
		Text:     "test",
		Data:     invitationData{InviterName: "Dana", OrgName: "Acme"},
	})
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	want := []string{
		"MAIL FROM:<alerts@example.com>",
		"RCPT TO:<ops@example.com>",
		"RCPT TO:<padded@example.com>",
		"RCPT TO:<plain@example.com>",
	}
	for i, w := range want {
		if i >= len(*captured) {
			t.Fatalf("missing command %d, got %v", i, *captured)
		}
		if (*captured)[i] != w {
			t.Errorf("command %d:\n got %q\nwant %q", i, (*captured)[i], w)
		}
	}
}
