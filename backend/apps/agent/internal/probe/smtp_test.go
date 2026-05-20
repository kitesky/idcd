package probe

import (
	"net"
	"testing"
	"time"
)

func TestSMTPProbe_badHost(t *testing.T) {
	p := &SMTPProbe{}
	result := p.Execute("192.0.2.1", 200*time.Millisecond, map[string]any{"port": "25"})
	if result.Success {
		t.Error("expected Success=false for unreachable host")
	}
	if result.Error == "" {
		t.Error("expected non-empty Error for unreachable host")
	}
}

func TestSMTPProbe_execute(t *testing.T) {
	// Start a mock TCP server that sends a 220 banner and responds to EHLO/QUIT.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte("220 mock.smtp.example ESMTP\r\n"))
		buf := make([]byte, 256)
		conn.Read(buf) // consume EHLO
		conn.Write([]byte("250-mock.smtp.example Hello\r\n"))
		conn.Read(buf) // consume QUIT
		conn.Write([]byte("221 Bye\r\n"))
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	p := &SMTPProbe{}
	result := p.Execute(host, 3*time.Second, map[string]any{"port": port})

	if !result.Success {
		t.Errorf("expected Success=true, got error: %s", result.Error)
	}
	banner, _ := result.Data["banner"].(string)
	if banner == "" {
		t.Error("expected non-empty banner in result data")
	}
}
