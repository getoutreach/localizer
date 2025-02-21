// Copyright 2022 Outreach Corporation. All Rights Reserved.
// Copyright 2020 Jared Allard
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Description: This file has the package ssh.
package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"strconv"

	"github.com/function61/gokit/io/bidipipe"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// This is based off of https://github.com/function61/holepunch-client
type Client struct {
	log logrus.FieldLogger

	// host of the remote SSH server
	host string

	// port of the remote SSH server
	port int

	// ports is the ports this client currently hosts
	// with the format being remotePort localPort
	ports map[uint]uint
}

// NewReverseTunnelClient creates a new ssh powered reverse
// tunnel client
func NewReverseTunnelClient(l logrus.FieldLogger, host string, port int, ports []string) *Client {
	portMap := make(map[uint]uint)
	for _, portStr := range ports {
		ports := strings.Split(portStr, ":")
		if len(ports) == 0 {
			return nil
		}

		localPort := 0
		remotePort := 0
		if len(ports) == 1 {
			portInt, err := strconv.Atoi(ports[0])
			if err != nil {
				panic(err)
			}

			localPort = portInt
			remotePort = portInt
		}

		if len(ports) == 2 {
			lport, err := strconv.Atoi(ports[0])
			if err != nil {
				panic(err)
			}

			rport, err := strconv.Atoi(ports[1])
			if err != nil {
				panic(err)
			}

			localPort = lport
			remotePort = rport
		}

		// nolint: gosec // Why: port numbers are never negative.
		portMap[uint(remotePort)] = uint(localPort)
	}
	return &Client{l, host, port, portMap}
}

// Start starts the ssh tunnel. This blocks until
// all listeners have closed
func (c *Client) Start(ctx context.Context, serviceKey string) error { //nolint:funlen // Why: there are no reusable parts to extract
	dialer := net.Dialer{
		Timeout: 10 * time.Second,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	sconn, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User: "outreach",
		Auth: []ssh.AuthMethod{
			// TODO(jaredallard): consider randomizing this?
			ssh.Password("supersecretpassword"),
		},
		//nolint:gosec // Why: We're not really caring about what we connect to here.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return err
	}

	sshClient := ssh.NewClient(sconn, chans, reqs)
	defer sshClient.Close()

	// send keep-alive messages
	// see: https://stackoverflow.com/questions/31554196/ssh-connection-timeout
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_, _, err := sshClient.Conn.SendRequest("keepalive@golang.org", true, nil)
				if err != nil {
					c.log.WithError(err).Warn("failed to send keep-alive")

					// recreate the connection
					cancel()
					return
				}
			}
		}
	}()

	wg := sync.WaitGroup{}
	for remotePort, localPort := range c.ports {
		// reverse listen on remote server port
		remoteAddr := fmt.Sprintf("0.0.0.0:%d", remotePort)
		localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
		listener, err := sshClient.Listen("tcp", remoteAddr)
		if err != nil {
			return errors.Wrapf(err, "failed to request remote to listen on %s", remoteAddr)
		}

		wg.Add(1)
		go func(remotePort uint) {
			defer listener.Close()
			defer wg.Done()

			c.log.Infof("created tunnel from remote %s:%d to %s", serviceKey, remotePort, localAddr)

			// handle incoming connections on reverse forwarded tunnel
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				client, err := listener.Accept()
				if err != nil {
					if !errors.Is(err, io.EOF) {
						c.log.WithError(err).Errorf("failed to accept traffic on remote listener")
					}
					return
				}

				// handle the connection in another goroutine, so we can support multiple concurrent
				// connections on the same port
				go c.handleReverseForwardConn(client, localAddr)
			}
		}(remotePort)
	}

	wg.Wait()

	return nil
}

func (c *Client) handleReverseForwardConn(client net.Conn, localAddr string) {
	defer client.Close()

	remote, err := net.Dial("tcp", localAddr)
	if err != nil {
		c.log.WithError(err).Errorf("failed to dial local service (is anything running on your machine at %q?)", localAddr)
		return
	}

	// pipe data in both directions:
	// - client => remote
	// - remote => client
	//
	// - in effect, we act as a proxy between the reverse tunnel's client and locally-dialed
	//   remote endpoint.
	// - the "client" and "remote" strings we give Pipe() is just for error&log messages
	// - this blocks until either of the parties' socket closes (or breaks)
	if err := bidipipe.Pipe(bidipipe.WithName("client", client), bidipipe.WithName("remote", remote)); err != nil {
		c.log.WithError(err).Warnf("failed to send data over tunnel")
	}
}
