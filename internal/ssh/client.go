package ssh

import (
	"context"
	"fmt"
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
			port, err := strconv.Atoi(ports[0])
			if err != nil {
				panic(err)
			}

			localPort = port
			remotePort = port
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

		portMap[uint(remotePort)] = uint(localPort)
	}
	return &Client{l, host, port, portMap}
}

func (c *Client) Start(ctx context.Context) error {
	dialer := net.Dialer{
		Timeout: 10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	sconn, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User: "outreach",
		Auth: []ssh.AuthMethod{
			// TODO(jaredallard): consider randomizing this?
			ssh.Password("supersecretpassword"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return err
	}

	sshClient := ssh.NewClient(sconn, chans, reqs)
	defer sshClient.Close()

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

			c.log.Infof("created tunnel from remote %d to %s", remotePort, localAddr)

			// handle incoming connections on reverse forwarded tunnel
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				client, err := listener.Accept()
				if err != nil {
					c.log.WithError(err).Errorf("failed to accept traffic on remote listener")
					return
				}

				// handle the connection in another goroutine, so we can support multiple concurrent
				// connections on the same port
				go c.handleReverseForwardConn(client, localAddr)
			}
		}(remotePort)
	}

	<-ctx.Done()
	wg.Wait()

	return nil
}

func (c *Client) handleReverseForwardConn(client net.Conn, localAddr string) {
	defer client.Close()

	remote, err := net.Dial("tcp", localAddr)
	if err != nil {
		c.log.WithError(err).Errorf("failed to dial local service")
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
		c.log.WithError(err).Errorf("tunnel failed")
	}
}
