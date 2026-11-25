package printer

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var gs1Payload = []byte("^XA\r\n^FO20,20^BXN,5,200,0,0,1,~^FD~10104600000000015\x1d21ABC\x00\xff~193TEST^FS\n^XZ\n")

func listen(t *testing.T) (string, <-chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got := make(chan []byte, 16)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			data, _ := io.ReadAll(conn)
			conn.Close()
			got <- data
		}
	}()
	return "tcp://" + ln.Addr().String(), got
}

func receive(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case data := <-ch:
		return data
	case <-time.After(5 * time.Second):
		t.Fatal("printer listener received nothing")
		return nil
	}
}

func TestTCPPayloadByteIdentical(t *testing.T) {
	uri, got := listen(t)
	p, err := Open(uri)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Print(context.Background(), Job{ID: "j1", Format: FormatZPL, Copies: 2, Data: gs1Payload}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if data := receive(t, got); !bytes.Equal(data, gs1Payload) {
			t.Fatalf("copy %d differs:\n got %q\nwant %q", i+1, data, gs1Payload)
		}
	}
}

func TestTCPConnectError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	p, err := Open("tcp://" + addr)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Print(context.Background(), Job{ID: "j", Format: FormatRaw, Data: []byte("x")}); err == nil {
		t.Fatal("expected connect error")
	}
}

func TestTCPDefaultPort(t *testing.T) {
	p, err := Open("tcp://printer.local")
	if err != nil {
		t.Fatal(err)
	}
	if p.URI() != "tcp://printer.local:9100" {
		t.Fatalf("uri %q", p.URI())
	}
}

func TestFilePayloadByteIdentical(t *testing.T) {
	dir := t.TempDir()
	p, err := Open("file://" + filepath.ToSlash(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Print(context.Background(), Job{ID: "job/42", Format: FormatZPL, Copies: 3, Data: gs1Payload}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"job_42-1.zpl", "job_42-2.zpl", "job_42-3.zpl"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, gs1Payload) {
			t.Fatalf("%s differs:\n got %q\nwant %q", name, data, gs1Payload)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Fatalf("expected 3 files, got %d", len(entries))
	}
}

func TestFileExtensions(t *testing.T) {
	dir := t.TempDir()
	p, err := Open("file://" + filepath.ToSlash(dir))
	if err != nil {
		t.Fatal(err)
	}
	for format, ext := range map[string]string{FormatPDF: "pdf", FormatRaw: "bin"} {
		if err := p.Print(context.Background(), Job{ID: format, Format: format, Data: []byte("%PDF")}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, format+"-1."+ext)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCUPSMissingLP(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p, err := Open("cups://Zebra_ZD420")
	if err != nil {
		t.Fatal(err)
	}
	err = p.Print(context.Background(), Job{ID: "j", Format: FormatZPL, Data: gs1Payload})
	if err == nil || !strings.Contains(err.Error(), "lp") {
		t.Fatalf("expected clear lp error, got %v", err)
	}
}

func TestCUPSPassesRawStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	script := "#!/bin/sh\necho \"$@\" > " + out + ".args\ncat > " + out + "\n"
	if err := os.WriteFile(filepath.Join(dir, "lp"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	p, err := Open("cups://Xprinter")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Print(context.Background(), Job{ID: "j7", Format: FormatZPL, Copies: 2, Data: gs1Payload}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if !bytes.Equal(data, gs1Payload) {
		t.Fatalf("lp stdin differs: %q", data)
	}
	args, _ := os.ReadFile(out + ".args")
	if got := strings.TrimSpace(string(args)); got != "-d Xprinter -o raw -n 2 -t j7" {
		t.Fatalf("lp args %q", got)
	}
}

func TestOpenRejects(t *testing.T) {
	for _, uri := range []string{"", "http://x", "tcp://", "cups://-o", "file://"} {
		if _, err := Open(uri); err == nil {
			t.Errorf("Open(%q) should fail", uri)
		}
	}
}

func TestCheckZPL(t *testing.T) {
	cases := map[string]bool{
		"^XA^FDx^FS^XZ":       true,
		"\r\n  ^XA^XZ":        true,
		"^FDx^XZ":             false,
		"^XA^FDno end":        false,
		"garbage ^XA ... ^XZ": false,
	}
	for in, ok := range cases {
		if err := CheckZPL([]byte(in)); (err == nil) != ok {
			t.Errorf("CheckZPL(%q) = %v, want ok=%v", in, err, ok)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(Job{Format: "png", Data: []byte("x")}); err == nil {
		t.Error("unknown format must fail")
	}
	if err := Validate(Job{Format: FormatPDF}); err == nil {
		t.Error("empty payload must fail")
	}
	if err := Validate(Job{Format: FormatRaw, Data: []byte{0x1d}}); err != nil {
		t.Error(err)
	}
}
