package main

import (
	"log"
	"net"
	"time"
	"flag"
	"strings"
	"os"
	"os/signal"
	"syscall"

	"github.com/miekg/dns"
)

var nameservers []string
var verbose *bool
var gateway *string

func getErrMsg(w dns.ResponseWriter, req *dns.Msg, err int) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(req)
	m.SetRcode(req, err)
	m.Authoritative = false
	m.RecursionAvailable = true
	return m
}

func getNsReply(w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
	if *verbose {
		log.Print("Forwarding ", req.Question[0].Name, "/", dns.Type(req.Question[0].Qtype).String())
	}

	client := &dns.Client{Net: "udp", ReadTimeout: 4 * time.Second, WriteTimeout: 4 * time.Second, SingleInflight: true}

	if _, tcp := w.RemoteAddr().(*net.TCPAddr); tcp {
		client.Net = "tcp"
	}

	var r *dns.Msg
	var err error

	for i := 0; i < len(nameservers); i++ {
		r, _, err = client.Exchange(req, nameservers[(int(req.Id) + i) % len(nameservers)])
		if err == nil {
			r.Compress = true
			return r
		}
	}

	log.Print("Failed to forward request.", err)
	return getErrMsg(w, req, dns.RcodeServerFailure)
}

func handleRequest(w dns.ResponseWriter, req *dns.Msg) {
	var m *dns.Msg

	if len(req.Question) > 0 && strings.HasSuffix(req.Question[0].Name, ".netflix.com.") {
		if req.Question[0].Qtype == dns.TypeA {
			m = getNsReply(w, req)
			for _, ans := range m.Answer {
				if ans.Header().Rrtype == dns.TypeA {
					ip := ans.(*dns.A).A.String()
					log.Print("Re-routing ", ip, " for ", ans.Header().Name, "/", dns.Type(ans.Header().Rrtype).String())
					// TODO reroute
					//"route add " + ip + "/32 " + *gateway
				} else if ans.Header().Rrtype == dns.TypeAAAA {
					// sanity check for now, shouldn't happen afaik
					log.Print("WARNING: AAAA response in ", ans.Header().Name, "/A")
				}
			}
		} else if req.Question[0].Qtype == dns.TypeAAAA {
			if *verbose {
				log.Print("Hijacking ", req.Question[0].Name, "/", dns.Type(req.Question[0].Qtype).String())
			}
			m = getErrMsg(w, req, dns.RcodeNameError)
		} else {
			m = getNsReply(w, req)
		}
	} else {
		m = getNsReply(w, req)
	}

	w.WriteMsg(m)
}

func main() {
	gateway = flag.String("gw", "192.168.1.1", "gateway IP to re-route to")
	verbose = flag.Bool("v", false, "verbose logging")

	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()

	log.Print("Starting DNS resolver...")

	nameservers = []string{"8.8.8.8:53", "8.8.4.4:53"}

	log.Print("Forwarding to ", nameservers)

	dns.HandleFunc(".", handleRequest)

	go func() {
		srv := &dns.Server{Addr: ":53", Net: "udp"}
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal("Failed to start UDP server.", err.Error())
		}
	}()

	go func() {
		srv := &dns.Server{Addr: ":53", Net: "tcp"}
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal("Failed to start TCP server.", err.Error())
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case s := <-sig:
			log.Fatalf("Received signal %d, exiting...", s)
		}
	}
}