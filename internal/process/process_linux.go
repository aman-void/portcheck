package process

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func inspect(ctx context.Context, host string, port int) ([]Info, error) {
	// /proc TCP tables do not expose the bound interface/scope. Refuse scoped
	// addresses rather than guessing between identical link-local addresses.
	if strings.Contains(host, "%") {
		return nil, fmt.Errorf("process inspection of scoped IPv6 addresses is unavailable")
	}
	ip, err := resolveHost(ctx, host)
	if err != nil {
		return nil, err
	}
	return inspectProc(ctx, "/proc", ip, port)
}

// Match Go's TCP bind resolver's IPv4 preference for hostnames. DNS may change
// between the bind and this independent observation; ownership is best effort.
func resolveHost(ctx context.Context, host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip, nil
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve inspection host: %w", err)
	}
	for _, ip := range ips {
		if ip.IP.To4() != nil {
			return ip.IP, nil
		}
	}
	if len(ips) == 0 {
		return nil, ErrNotFound
	}
	return ips[0].IP, nil
}

func inspectProc(ctx context.Context, root string, ip net.IP, port int) ([]Info, error) {
	inodes := make(map[string]bool)
	for _, table := range []string{"tcp", "tcp6"} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, err := os.Open(filepath.Join(root, "net", table))
		if errors.Is(err, os.ErrNotExist) && table == "tcp6" {
			continue
		}
		if err != nil {
			return nil, procError(err)
		}
		found, readErr := listenerInodes(ctx, f, ip, port)
		closeErr := f.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return nil, procError(err)
		}
		for inode := range found {
			inodes[inode] = true
		}
	}
	if len(inodes) == 0 {
		return nil, ErrNotFound
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, procError(err)
	}
	var owners []Info
	denied := false
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		owns, err := ownsSocket(ctx, dir, inodes)
		if err != nil {
			denied = denied || errors.Is(err, os.ErrPermission)
			if !transient(err) {
				return nil, procError(err)
			}
			continue
		}
		if !owns {
			continue
		}
		// Read metadata only for candidate owners. Verify identity across these
		// reads; disappearing entries are ordinary process races.
		before, err := processStart(dir)
		if err != nil {
			denied = denied || errors.Is(err, os.ErrPermission)
			if !transient(err) {
				return nil, procError(err)
			}
			continue
		}
		info := Info{PID: pid}
		name, nameErr := os.ReadFile(filepath.Join(dir, "comm"))
		if nameErr == nil {
			info.Name = strings.TrimSuffix(string(name), "\n")
		}
		info.Executable, _ = os.Readlink(filepath.Join(dir, "exe"))
		after, err := processStart(dir)
		if err != nil {
			denied = denied || errors.Is(err, os.ErrPermission)
			if !transient(err) {
				return nil, procError(err)
			}
			continue
		}
		if before != after {
			continue
		}
		// Recheck socket ownership after reading metadata; do not trust a stale
		// fd symlink from before a close or process exit.
		owns, err = ownsSocket(ctx, dir, inodes)
		if err != nil {
			denied = denied || errors.Is(err, os.ErrPermission)
			if !transient(err) {
				return nil, procError(err)
			}
			continue
		}
		if owns {
			owners = append(owners, info)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i].PID < owners[j].PID })
	if len(owners) > 0 {
		return owners, nil
	}
	if denied {
		return nil, ErrPermission
	}
	return nil, ErrNotFound
}

func transient(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission)
}

func procError(err error) error {
	if errors.Is(err, os.ErrPermission) {
		return errors.Join(ErrPermission, err)
	}
	return fmt.Errorf("read process information: %w", err)
}

func processStart(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return "", err
	}
	// comm in parentheses may contain spaces and closing parentheses.
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return "", fmt.Errorf("malformed process stat")
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 20 {
		return "", fmt.Errorf("malformed process stat")
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return "", fmt.Errorf("malformed process start time: %w", err)
	}
	return fields[19], nil // stat field 22; fields starts at field 3.
}

func ownsSocket(ctx context.Context, dir string, inodes map[string]bool) (bool, error) {
	fds, err := os.ReadDir(filepath.Join(dir, "fd"))
	if err != nil {
		return false, err
	}
	for _, fd := range fds {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		target, err := os.Readlink(filepath.Join(dir, "fd", fd.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") && inodes[target[8:len(target)-1]] {
			return true, nil
		}
	}
	return false, nil
}

func listenerInodes(ctx context.Context, r io.Reader, requested net.IP, port int) (map[string]bool, error) {
	inodes := make(map[string]bool)
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return nil, errors.Join(errors.New("missing TCP table header"), scanner.Err())
	}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			return nil, fmt.Errorf("malformed TCP table row")
		}
		if fields[3] != "0A" {
			continue
		} // LISTEN only, never established peers.
		address, hexPort, ok := strings.Cut(fields[1], ":")
		p, err := strconv.ParseUint(hexPort, 16, 16)
		if !ok || err != nil {
			return nil, fmt.Errorf("malformed TCP table port")
		}
		if int(p) != port {
			continue
		}
		ip, err := procIP(address)
		if err != nil {
			return nil, err
		}
		if !conflicts(requested, ip) {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("malformed socket inode")
		}
		if inode != 0 {
			inodes[fields[9]] = true
		}
	}
	return inodes, scanner.Err()
}

// proc writes each 32-bit address word in native byte order, including IPv6.
func procIP(value string) (net.IP, error) {
	if len(value) != 8 && len(value) != 32 {
		return nil, fmt.Errorf("malformed TCP table address")
	}
	data, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("malformed TCP table address: %w", err)
	}
	for i := 0; i < len(data); i += 4 {
		binary.BigEndian.PutUint32(data[i:i+4], binary.NativeEndian.Uint32(data[i:i+4]))
	}
	return net.IP(data), nil
}

// Match only within an address family. /proc does not expose IPV6_V6ONLY,
// so attributing a cross-family conflict would risk reporting an unrelated PID.
func conflicts(requested, listener net.IP) bool {
	if (requested.To4() != nil) != (listener.To4() != nil) {
		return false
	}
	return requested.IsUnspecified() || listener.IsUnspecified() || requested.Equal(listener)
}
