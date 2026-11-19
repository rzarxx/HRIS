package filescan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClamAVResponses(t *testing.T) {
	for _, tc := range []struct {
		name, reply   string
		clean, unsafe bool
	}{
		{"clean", "stream: OK\x00", true, false},
		{"infected", "stream: Eicar-Signature FOUND\x00", false, true},
		{"encrypted", "stream: Heuristics.Encrypted.PDF FOUND\x00", false, true},
		{"limit", "INSTREAM size limit exceeded. ERROR\x00", false, false},
		{"truncated", "stream: OK", false, false},
		{"unknown", "OK\x00", false, false},
		{"oversize reply", strings.Repeat("x", 5000) + "\x00", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			input := bytes.Repeat([]byte("private document"), 10000)
			done := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				r := bufio.NewReader(conn)
				command, err := r.ReadString(0)
				if err != nil || command != "zINSTREAM\x00" {
					done <- errors.New("invalid command")
					return
				}
				var received bytes.Buffer
				for {
					var n uint32
					if err := binary.Read(r, binary.BigEndian, &n); err != nil {
						done <- err
						return
					}
					if n == 0 {
						break
					}
					if n > 65536 {
						done <- errors.New("oversized chunk")
						return
					}
					if _, err := io.CopyN(&received, r, int64(n)); err != nil {
						done <- err
						return
					}
				}
				if !bytes.Equal(received.Bytes(), input) {
					done <- errors.New("changed scan bytes")
					return
				}
				_, _ = io.WriteString(conn, tc.reply)
				done <- nil
			}()
			scanner, err := New(listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			err = scanner.Scan(context.Background(), input)
			if (err == nil) != tc.clean || errors.Is(err, ErrUnsafe) != tc.unsafe {
				t.Fatalf("unexpected scan result: %v", err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClamAVCancelled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			_, _ = io.Copy(io.Discard, conn)
		}
	}()
	scanner, _ := New(listener.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := scanner.Scan(ctx, []byte("document")); err == nil {
		t.Fatal("timeout must fail closed")
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation did not close scan")
	}
}

func TestAddressMustBeLocal(t *testing.T) {
	for _, address := range []string{"", "example.com:3310", "192.0.2.1:3310", "127.0.0.1:0", "127.0.0.1:65536"} {
		if _, err := New(address); err == nil {
			t.Fatalf("accepted %q", address)
		}
	}
	for _, address := range []string{"127.0.0.1:3310", "[::1]:3310"} {
		if _, err := New(address); err != nil {
			t.Fatal(err)
		}
	}
}
