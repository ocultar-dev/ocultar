package proxy

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/ocultar-dev/ocultar/pkg/refinery"
)

// SyslogServer listens for incoming UDP syslog messages, redacts PII,
// and forwards them to an upstream SIEM.
type SyslogServer struct {
	eng          *refinery.Refinery
	listenAddr   string
	upstreamAddr string
	conn         *net.UDPConn
}

// NewSyslogServer creates a new Syslog proxy server
func NewSyslogServer(eng *refinery.Refinery, listenAddr, upstreamAddr string) *SyslogServer {
	return &SyslogServer{
		eng:          eng,
		listenAddr:   listenAddr,
		upstreamAddr: upstreamAddr,
	}
}

// Start opens the UDP listener and starts processing messages
func (s *SyslogServer) Start() error {
	addr, err := net.ResolveUDPAddr("udp", s.listenAddr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	var upstream *net.UDPAddr
	if s.upstreamAddr != "" {
		upstream, err = net.ResolveUDPAddr("udp", s.upstreamAddr)
		if err != nil {
			return fmt.Errorf("invalid upstream syslog addr: %w", err)
		}
	}

	go s.listen(upstream)
	return nil
}

func (s *SyslogServer) listen(upstream *net.UDPAddr) {
	buf := make([]byte, 65535)
	for {
		n, _, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			slog.Error("syslog read error", "error", err)
			continue
		}

		msg := string(buf[:n])
		// Send through refinery for redaction
		refined, err := s.eng.RefineString(msg, "syslog_proxy", nil)
		if err != nil {
			slog.Warn("syslog refinery error, dropping message (fail-closed)", "error", err)
			continue // Fail-closed: drop on error
		}

		if upstream != nil {
			// Forward to SIEM
			upstreamConn, err := net.DialUDP("udp", nil, upstream)
			if err == nil {
				if _, err := upstreamConn.Write([]byte(refined)); err != nil {
					slog.Error("syslog failed to send to SIEM", "error", err)
				}
				upstreamConn.Close() //nolint:errcheck
			} else {
				slog.Error("syslog failed to dial upstream", "error", err)
			}
		} else {
			// Log locally if no upstream SIEM is configured
			slog.Info("syslog message (redacted)", "message", refined)
		}
	}
}

// Stop closes the UDP connection
func (s *SyslogServer) Stop() {
	if s.conn != nil {
		s.conn.Close() //nolint:errcheck
	}
}
