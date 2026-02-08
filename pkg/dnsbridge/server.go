package dnsbridge

import (
	"fmt"
	"strings"

	"github.com/miekg/dns"
	"github.com/nathfavour/zdns/pkg/ipc"
)

type Server struct {
	Addr      string
	GetStatus func() []ipc.PeerStatus
}

func NewServer(addr string) *Server {
	return &Server{
		Addr: addr,
	}
}

func (s *Server) Start() error {
	dns.HandleFunc("zdns.", s.handleDNSRequest)
	server := &dns.Server{Addr: s.Addr, Net: "udp"}
	fmt.Printf("DNS Bridge active at %s (Resolving .zdns)\n", s.Addr)
	return server.ListenAndServe()
}

func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	if r.Opcode == dns.OpcodeQuery {
		for _, q := range m.Question {
			if q.Qtype == dns.TypeA {
				name := strings.TrimSuffix(q.Name, ".zdns.")
				ip := s.resolvePeer(name)
				if ip != "" {
					rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
					if err == nil {
						m.Answer = append(m.Answer, rr)
					}
				}
			}
		}
	}

	w.WriteMsg(m)
}

func (s *Server) resolvePeer(name string) string {
	if s.GetStatus == nil {
		return ""
	}
	peers := s.GetStatus()
	for _, p := range peers {
		if strings.EqualFold(p.Name, name) {
			return p.IP
		}
	}
	return ""
}
