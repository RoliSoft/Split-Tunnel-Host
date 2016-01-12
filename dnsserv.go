package main

import (
	"log"
	"net"
	"time"
	"flag"
	"path"
	"strings"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/miekg/dns"
)

var nameservers []string
var verbose *bool
var gateway4 *string
var gateway6 *string
var routev6 bool
var router string
var routedv4 map[string]struct{}
var routedv6 map[string]struct{}

func runAndLog(name string, arg ...string) {
	out, err := exec.Command(name, arg...).CombinedOutput()

	if err != nil {
		if len(out) > 1 {
			log.Printf("%s: %s", path.Base(strings.Replace(name, "\\", "/", -1)), out)
		} else {
			log.Print(err)
		}
	} else if *verbose {
		log.Printf("%s: %s", path.Base(strings.Replace(name, "\\", "/", -1)), out)
	}
}

func getEmptyMsg(w dns.ResponseWriter, req *dns.Msg, err int) *dns.Msg {
	m := new(dns.Msg)

	m.SetReply(req)

	if err != 0 {
		m.SetRcode(req, err)
	}

	m.Authoritative = false
	m.RecursionAvailable = true

	return m
}

func getServerReply(w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
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
	return getEmptyMsg(w, req, dns.RcodeServerFailure)
}

func handleRequest(w dns.ResponseWriter, req *dns.Msg) {
	var m *dns.Msg

	if len(req.Question) > 0 && (req.Question[0].Name == "netflix.com." || strings.HasSuffix(req.Question[0].Name, ".netflix.com.")) {
		if req.Question[0].Qtype == dns.TypeA {
			m = getServerReply(w, req)
			for _, ans := range m.Answer {
				if ans.Header().Rrtype == dns.TypeA {
					ip := ans.(*dns.A).A.String()
					routedv4[ip] = struct{}{}

					log.Print("Re-routing ", ip, " for ", ans.Header().Name, "/", dns.Type(ans.Header().Rrtype).String())

					runAndLog(router, "add", ip + "/32", *gateway4)
				} else if ans.Header().Rrtype == dns.TypeAAAA {
					// sanity check for now, shouldn't happen afaik
					log.Print("WARNING: AAAA response in ", ans.Header().Name, "/A")
				}
			}
		} else if req.Question[0].Qtype == dns.TypeAAAA {
			if routev6 {
				m = getServerReply(w, req)
				for _, ans := range m.Answer {
					if ans.Header().Rrtype == dns.TypeAAAA {
						ip := ans.(*dns.AAAA).AAAA.String()
						routedv6[ip] = struct{}{}

						log.Print("Re-routing ", ip, " for ", ans.Header().Name, "/", dns.Type(ans.Header().Rrtype).String())

						runAndLog(router, "add", ip + "/128", *gateway6)
					} else if ans.Header().Rrtype == dns.TypeA {
						log.Print("WARNING: A response in ", ans.Header().Name, "/AAAA")
					}
				}
			} else {
				if *verbose {
					log.Print("Hijacking ", req.Question[0].Name, "/", dns.Type(req.Question[0].Qtype).String())
				}
				m = getEmptyMsg(w, req, 0)
			}
		} else {
			m = getServerReply(w, req)
		}
	} else {
		m = getServerReply(w, req)
	}

	w.WriteMsg(m)
}

func removeRoutes() {
	if len(routedv4) > 0 {
		log.Print("Removing routes...")

		for ip, _ := range routedv4 {
			runAndLog(router, "delete", ip + "/32")
		}
	}

	if routev6 && len(routedv6) > 0 {
		log.Print("Removing IPv6 routes...")

		for ip, _ := range routedv6 {
			runAndLog(router, "delete", ip + "/128")
		}
	}
}

func main() {
	gateway4 = flag.String("r", "", "IPv4 gateway address for routing destination")
	gateway6 = flag.String("r6", "", "IPv6 gateway address for routing destination")
	verbose = flag.Bool("v", false, "verbose logging")

	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()

	router, _ = exec.LookPath("route")
	if len(router) < 1 {
		log.Fatal("Unable to find the `route` command in your %PATH%.")
	}

	if len(*gateway4) < 1 {
		log.Fatal("A gateway IP must be specified via argument `r`.")
	}

	if ip := net.ParseIP(*gateway4); ip != nil && ip.To4() != nil {
		*gateway4 = ip.String()
	} else {
		log.Fatal("Specified gateway IP is not a valid IPv4 address.")
	}

	if len(*gateway6) > 1 {
		if ip := net.ParseIP(*gateway6); ip != nil && ip.To4() == nil {
			*gateway6 = ip.String()
			  routev6 = true
		} else {
			log.Fatal("Specified gateway IP is not a valid IPv6 address.")
		}
	} else {
		routev6 = false
	}

	routedv4 = make(map[string]struct{})

	if (routev6) {
		routedv6 = make(map[string]struct{})
	}

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
			removeRoutes()
			log.Fatalf("Received signal %d, exiting...", s)
		}
	}
}