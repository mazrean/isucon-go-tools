package isuhttp

import (
	"fmt"
	"net"
	"os"
)

func newUnixDomainSockListener() (net.Listener, bool, error) {
	sockPath, ok := os.LookupEnv("UNIX_SOCKET")
	if !ok {
		return nil, false, nil
	}

	err := os.Remove(sockPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("failed to remove socket file: %w", err)
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, false, fmt.Errorf("unix domain sock listen error: %w", err)
	}

	err = os.Chmod(sockPath, 0777)
	if err != nil {
		listener.Close()
		return nil, false, fmt.Errorf("unix domain sock chmod error: %w", err)
	}

	return listener, true, nil
}

func newTCPListener(addr string) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return listener, nil
}
