// Package filescan scans private uploads through a local ClamAV daemon.
package filescan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var ErrUnsafe = errors.New("file rejected by malware scanner")

type Scanner interface {
	Scan(context.Context, []byte) error
}

type ClamAV struct {
	network, address string
	slots            chan struct{}
}

// New permits only a local socket or a literal loopback IP: clamd has no TLS/auth.
func New(address string) (*ClamAV, error) {
	if filepath.IsAbs(address) {
		return &ClamAV{network: "unix", address: address, slots: make(chan struct{}, 2)}, nil
	}
	host, port, err := net.SplitHostPort(address)
	p, portErr := strconv.Atoi(port)
	ip := net.ParseIP(host)
	if err != nil || portErr != nil || p < 1 || p > 65535 || ip == nil || !ip.IsLoopback() {
		return nil, errors.New("CLAMAV_ADDRESS harus socket absolut atau IP loopback:port")
	}
	return &ClamAV{network: "tcp", address: address, slots: make(chan struct{}, 2)}, nil
}

// Scan accepts only a complete, exact OK response. Timeouts, truncated replies,
// limits and daemon errors never count as a clean scan. No uploaded bytes are logged.
func (c *ClamAV) Scan(ctx context.Context, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, c.network, c.address)
	if err != nil {
		return fmt.Errorf("connect scanner: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	if _, err := io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return err
	}
	for offset := 0; offset < len(data); {
		end := min(offset+64*1024, len(data))
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(end-offset))
		if _, err := io.Copy(conn, bytes.NewReader(size[:])); err != nil {
			return err
		}
		if _, err := io.Copy(conn, bytes.NewReader(data[offset:end])); err != nil {
			return err
		}
		offset = end
	}
	if _, err := io.Copy(conn, bytes.NewReader(make([]byte, 4))); err != nil {
		return err
	}
	reply, err := bufio.NewReader(io.LimitReader(conn, 4096)).ReadString(0)
	if err != nil {
		return fmt.Errorf("scanner response: %w", err)
	}
	reply = strings.TrimSuffix(reply, "\x00")
	if reply == "stream: OK" {
		return nil
	}
	if strings.HasPrefix(reply, "stream: ") && strings.HasSuffix(reply, " FOUND") {
		return ErrUnsafe
	}
	return errors.New("scanner did not confirm a clean file")
}
