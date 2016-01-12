package main

import (
	"log"
	"net"
	"time"
	"flag"
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
var routedv4 []string
var routedv6 []string

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
	return getEmptyMsg(w, req, dns.RcodeServerFailure)
}

func handleRequest(w dns.ResponseWriter, req *dns.Msg) {
	var m *dns.Msg

	if len(req.Question) > 0 && (req.Question[0].Name == "netflix.com." || strings.HasSuffix(req.Question[0].Name, ".netflix.com.")) {
		if req.Question[0].Qtype == dns.TypeA {
			m = getNsReply(w, req)
			for _, ans := range m.Answer {
				if ans.Header().Rrtype == dns.TypeA {
					ip := ans.(*dns.A).A.String()
					routedv4 = append(routedv4, ip)

					log.Print("Re-routing ", ip, " for ", ans.Header().Name, "/", dns.Type(ans.Header().Rrtype).String())

					out, err := exec.Command(router, "add", ip + "/32", *gateway4).CombinedOutput()
					if err != nil {
						log.Print(err)
					} else if *verbose {
						log.Printf("route: %s", out)
					}
				} else if ans.Header().Rrtype == dns.TypeAAAA {
					// sanity check for now, shouldn't happen afaik
					log.Print("WARNING: AAAA response in ", ans.Header().Name, "/A")
				}
			}
		} else if req.Question[0].Qtype == dns.TypeAAAA {
			if routev6 {
				m = getNsReply(w, req)
				for _, ans := range m.Answer {
					if ans.Header().Rrtype == dns.TypeAAAA {
						ip := ans.(*dns.AAAA).AAAA.String()
						routedv6 = append(routedv6, ip)

						log.Print("Re-routing ", ip, " for ", ans.Header().Name, "/", dns.Type(ans.Header().Rrtype).String())

						out, err := exec.Command(router, "add", ip + "/128", *gateway6).CombinedOutput()
						if err != nil {
							log.Print(err)
						} else if *verbose {
							log.Printf("route: %s", out)
						}
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
			m = getNsReply(w, req)
		}
	} else {
		m = getNsReply(w, req)
	}

	w.WriteMsg(m)
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
			if len(routedv4) > 0 {
				log.Print("Removing routes...")

				for _, ip := range routedv4 {
					out, err := exec.Command(router, "delete", ip + "/32").CombinedOutput()
					if err != nil {
						log.Print(err)
					} else if *verbose {
						log.Printf("route: %s", out)
					}
				}
			}

			if routev6 && len(routedv6) > 0 {
				log.Print("Removing IPv6 routes...")

				for _, ip := range routedv6 {
					out, err := exec.Command(router, "delete", ip + "/128").CombinedOutput()
					if err != nil {
						log.Print(err)
					} else if *verbose {
						log.Printf("route: %s", out)
					}
				}
			}

			log.Fatalf("Received signal %d, exiting...", s)
		}
	}
}