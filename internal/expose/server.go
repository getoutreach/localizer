package expose

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/google/uuid"
	v1 "github.com/jaredallard/localizer/api/localizer/v1"
	"github.com/sirupsen/logrus"
)

type Server struct {
	log logrus.FieldLogger

	grcpConnsMutex sync.Mutex
	tcpConnsMutex  sync.Mutex

	tcpConns  map[uint]map[string]net.Conn
	listeners map[uint]net.Listener
	grpcConns map[uint]v1.TunnelService_TunnelServer
}

func NewServer(log logrus.FieldLogger) *Server {
	return &Server{
		log:       log,
		tcpConns:  make(map[uint]map[string]net.Conn),
		listeners: make(map[uint]net.Listener),
		grpcConns: make(map[uint]v1.TunnelService_TunnelServer),
	}
}

// manageTCPServer handles
func (s *Server) manageTCPServer(l net.Listener, port uint) {
	buff := make([]byte, 10000)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			s.log.WithError(err).Error("failed to accept connection")
			continue
		}

		// if we don't have a connection on that port we are sad :(
		ts := s.grpcConns[port]
		if ts == nil {
			conn.Close()
			return
		}

		// connection exists, we start tracking the client
		s.tcpConnsMutex.Lock()
		if s.tcpConns[port] == nil {
			s.tcpConns[port] = make(map[string]net.Conn)
		}

		u := uuid.New()

		s.tcpConns[port][u.String()] = conn
		s.tcpConnsMutex.Unlock()

		s.log.WithFields(logrus.Fields{
			"port": port,
		}).Info("reading data from remote (tcp)")

		bytesRead, err := conn.Read(buff)
		if err != nil {
			if err != io.EOF {
				s.log.WithError(err).Error("error occurred while reading from TCP connection")
			}
			return
		}

		s.log.WithFields(logrus.Fields{
			"port":  port,
			"bytes": bytesRead,
		}).Info("sending data from remote (tcp) to grpc (local)")

		err = ts.Send(&v1.Chunk{
			Data: buff[0:bytesRead],
		})
		if err != nil {
			s.log.WithError(err).Error("failed to send data to grpc client")
		}
	}
}

// Tunnel tunnels TCP traffic over GRPC
// Note: This was heavily inspired by" https://github.com/diamondo25/grpc-tcp-tunnel
func (s *Server) Tunnel(ts v1.TunnelService_TunnelServer) error {
	errChan := make(chan error)

	// client (local->remote) -> remote port
	go func() {
		for {
			c, err := ts.Recv()
			if err != nil {
				if err != io.EOF {
					s.log.WithError(err).Error("error encountered while writing to GRPC client")
				}
				errChan <- nil
				return
			}

			port := uint(c.Port)

			// client is sending a handshake to open a new port
			// or attach to an existing listener
			if c.Event == v1.Event_EVENT_START_ACCEPTING {
				s.grcpConnsMutex.Lock()
				if _, exists := s.grpcConns[port]; !exists {
					// create the proxy, since we don't have a listener registered

					// bind to the port specified by the remote, if we don't already
					// have a listener. If we do, then just replace the client
					if s.listeners[port] != nil {
						l, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Port))
						if err != nil {
							s.log.WithError(err).Error("failed to listen on port")
							return
						}
						s.listeners[port] = l
					}

					// set this connection as in charge of this port
					s.grpcConns[port] = ts

					// proxy data from this port to this grpc conn
					go s.manageTCPServer(l, port)
				}
				s.grcpConnsMutex.Unlock()

				// since we processed an event, we skip this packet
				continue
			}

			data := c.Data

			s.log.Info("sending bytes over port", len(data))

			// skip requests without an Id
			if c.Id == "" {
				continue
			}

			// if we don't know of the ID, or the port isn't active, skip the request
			if s.tcpConns[port] == nil || s.tcpConns[port][c.Id] == nil {
				continue
			}

			_, err = s.tcpConns[port][c.Id].Write(data)
			if err != nil {
				s.log.WithError(err).Error("failed to write data tcp connection")
				continue
			}
		}
	}()

	// Blocking read
	returnedError := <-errChan

	return returnedError
}
