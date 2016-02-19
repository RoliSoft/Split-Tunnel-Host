// Copyright 2016 RoliSoft.  All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

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
	"runtime"
)

var (
	nameservers []string
	verbose      *bool
	gateway4     *string
	gateway6     *string
	dnsr1        *string
	dnsr2        *string
	routev6       bool
	router        string
	routedv4      map[string]struct{}
	routedv6      map[string]struct{}
)

// Executes the specified command with the specified arguments. The
// `name` parameter should be an absolute path to the executable.
// See `exec.LookPath()` for resolving names found in `%PATH%`.
// Any errors will be logged to stdout, if an output is available,
// otherwise the return code or internal Go error will be displayed.
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

// Creates an empty response to the specified request. If `err` is
// specified, the `RCODE` field in the response will be set to this value.
// If `err` is set to 0, the `RCODE` field will not be modified, and the
// resulting packet will just mean that the domain exists (not `NXDOMAIN`)
// but there are no records of the requested type associated to it.
// If `NXDOMAIN` is sent as a reply to the hijacked `AAAA` records of hostnames
// when IPv6 routing is disabled, some web browsers (e.g. Chrome) will display
// an error message stating `DNS_PROBE_FINISHED_NXDOMAIN`, even though a request
// for `A` records types is sent and properly replied to by the server to the client.
// Even though the original `ResponseWriter` object is taken as an argument,
// this function does not send a reply to the client. Instead, the
// packet is returned for further processing by the caller.
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

// Forwards a DNS request to the specified nameservers. On success, the
// original reply packet will be returned to the caller. On failure, a
// new packet will be returned with `RCODE` set to `SERVFAIL`.
// Even though the original `ResponseWriter` object is taken as an argument,
// this function does not send a reply to the client. Instead, the
// packet is returned for further processing by the caller.
func getServerReply(w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
	if *verbose {
		log.Print("Forwarding ", req.Question[0].Name, "/", dns.Type(req.Question[0].Qtype).String())
	}

	// create a new DNS client

	client := &dns.Client{Net: "udp", ReadTimeout: 4 * time.Second, WriteTimeout: 4 * time.Second, SingleInflight: true}

	if _, tcp := w.RemoteAddr().(*net.TCPAddr); tcp {
		client.Net = "tcp"
	}

	var r *dns.Msg
	var err error

	// loop through the specified nameservers and forward them the request

	// the request ID is used as a starting point in order to introduce at least
	// some element of randomness, instead of always hitting the first nameserver

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

// Checks whether the specified hostname is part of the zone which
// is set to be hijacked or whose IP addresses require split-tunneling.
func isTargetZone(name string) bool {
	return name == "netflix.com." || strings.HasSuffix(name, ".netflix.com.")
}

// Handles requests with questions for records of type `A`.
// Forwards the request to the specified nameservers and returns the
// result to the caller to be sent back to the client. However, before
// a reply is sent back, the IP addresses found within the response
// packet are added to the routing table.
func handleV4Hijack(w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
	// forward request to the specified nameservers

	m := getServerReply(w, req)

	// iterate through the answers in the reply and handle based on type

	for _, ans := range m.Answer {
		if ans.Header().Rrtype == dns.TypeA {
			// add IP address to local list and then routing table

			ip := ans.(*dns.A).A.String()
			routedv4[ip] = struct{}{}

			log.Print("Re-routing ", ip, " for ", ans.Header().Name, "/", dns.Type(ans.Header().Rrtype).String())

			if runtime.GOOS == "windows" {
				runAndLog(router, "add", ip + "/32", *gateway4)
			} else {
				runAndLog(router, "add", ip + "/32", "gw", *gateway4)
			}
		} else if ans.Header().Rrtype == dns.TypeAAAA {
			// sanity check for now, shouldn't happen afaik

			log.Print("WARNING: AAAA response in ", ans.Header().Name, "/A")
		}
	}

	return m
}

// Handles requests with questions for records of type `AAAA`.
// If IPv6 routing is enabled, the request will undergo the same
// treatment as `A` types in `handleV4Hijack()`; otherwise an empty
// packet is sent back as a reply.
func handleV6Hijack(w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
	var m *dns.Msg

	// if an IPv6 gateway address was specified, AAAA records will be forwarded
	// and processed; otherwise a fake reply is sent back telling the user agent
	// that there are no IPv6 addresses present at this address

	if routev6 {
		// forward request to the specified nameservers

		m = getServerReply(w, req)

		// iterate through the answers in the reply and handle based on type

		for _, ans := range m.Answer {
			if ans.Header().Rrtype == dns.TypeAAAA {
				// add IP address to local list and then routing table

				ip := ans.(*dns.AAAA).AAAA.String()
				routedv6[ip] = struct{}{}

				log.Print("Re-routing ", ip, " for ", ans.Header().Name, "/", dns.Type(ans.Header().Rrtype).String())

				if runtime.GOOS == "windows" {
					runAndLog(router, "add", ip + "/128", *gateway6)
				} else {
					runAndLog(router, "add", ip + "/128", "gw", *gateway6)
				}
			} else if ans.Header().Rrtype == dns.TypeA {
				// sanity check for now, shouldn't happen afaik

				log.Print("WARNING: A response in ", ans.Header().Name, "/AAAA")
			}
		}
	} else {
		if *verbose {
			log.Print("Hijacking ", req.Question[0].Name, "/", dns.Type(req.Question[0].Qtype).String())
		}

		// create new empty reply packet with no errors indicated

		// if `RCODE` is set to `NXDOMAIN` instead of the current empty packet method,
		// some web browsers (e.g. Chrome) will display an error message stating
		// `DNS_PROBE_FINISHED_NXDOMAIN`, even though a request for `A` records types is
		// sent by the user agent and properly replied to (with addresses) by the server

		m = getEmptyMsg(w, req, 0)
	}

	return m
}

// Handles an incoming DNS request packet. This function decides whether
// the hostname listed in the DNS packet is worthy of manipulation, or
// not. The IP addresses listed in the reply to the user for a target
// hostname are added to the routing table at this time before a
// reply is sent back to the user, otherwise the user agent of the client
// might connect faster than the routing changes can be made.
func handleRequest(w dns.ResponseWriter, req *dns.Msg) {
	var m *dns.Msg

	// check if the the hostname in the request matches the target

	if len(req.Question) > 0 && isTargetZone(req.Question[0].Name) {
		// handle `A` and `AAAA` types accordingly
		// other record types will be forwarded without manipulation

		switch req.Question[0].Qtype {

		case dns.TypeA:
			m = handleV4Hijack(w, req)

		case dns.TypeAAAA:
			m = handleV6Hijack(w, req)

		}
	}

	// if no reply was previously set, forward it

	if m == nil {
		m = getServerReply(w, req)
	}

	// send reply back to user

	w.WriteMsg(m)
}

// Removes the routes from the operating system's routing table that have been
// added during the lifetime of the server. Failure to call this function
// during exit may result in the inaccessibility of the added IP addresses.
func removeRoutes() {
	// remove IPv4 routes

	if len(routedv4) > 0 {
		log.Print("Removing routes...")

		for ip := range routedv4 {
			if runtime.GOOS == "windows" {
				runAndLog(router, "delete", ip + "/32")
			} else {
				runAndLog(router, "del", ip + "/32")
			}
		}
	}

	// remove IPv6 routes

	if routev6 && len(routedv6) > 0 {
		log.Print("Removing IPv6 routes...")

		for ip := range routedv6 {
			if runtime.GOOS == "windows" {
				runAndLog(router, "delete", ip + "/128")
			} else {
				runAndLog(router, "del", ip + "/128")
			}
		}
	}
}

// Main entry point of the application. Its behaviour can be modified
// via command line arguments as shown by the `flag` calls inside.
func main() {
	// set-up flags

	gateway4 = flag.String("r", "", "IPv4 gateway address for routing destination")
	gateway6 = flag.String("r6", "", "IPv6 gateway address for routing destination")
	dnsr1    = flag.String("dp", "8.8.8.8", "primary DNS server")
	dnsr2    = flag.String("ds", "8.8.4.4", "secondary DNS server")
	verbose  = flag.Bool("v", false, "verbose logging")

	flag.Usage = func() {
		flag.PrintDefaults()
	}

	flag.Parse()

	// find the route utility in %PATH%

	router, _ = exec.LookPath("route")
	if len(router) < 1 {
		log.Fatal("Unable to find the `route` command in your PATH env var.")
	}

	// read gateway from arguments

	if len(*gateway4) < 1 {
		log.Fatal("A gateway IP must be specified via argument `r`.")
	}

	if ip := net.ParseIP(*gateway4); ip != nil && ip.To4() != nil {
		*gateway4 = ip.String()
	} else {
		log.Fatal("Specified gateway IP is not a valid IPv4 address.")
	}

	// IPv6 gateway is optional

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

	// allocate set for routed IP addresses

	routedv4 = make(map[string]struct{})

	if (routev6) {
		routedv6 = make(map[string]struct{})
	}

	// read DNS servers to forward to

	if ip := net.ParseIP(*dnsr1); ip != nil {
		*dnsr1 = ip.String()
	} else {
		log.Fatal("Specified primary DNS server is not a valid IP address.")
	}

	if ip := net.ParseIP(*dnsr2); ip != nil {
		*dnsr2 = ip.String()
	} else {
		log.Fatal("Specified secondary DNS server is not a valid IP address.")
	}

	nameservers = []string{*dnsr1 + ":53", *dnsr2 + ":53"}

	// start DNS server

	log.Print("Starting DNS resolver...")
	log.Print("Forwarding to ", nameservers)

	dns.HandleFunc(".", handleRequest)

	// start local UDP listener

	go func() {
		srv := &dns.Server{Addr: ":53", Net: "udp"}
		err := srv.ListenAndServe()

		if err != nil {
			log.Fatal("Failed to start UDP server.", err.Error())
		}
	}()

	// start local TCP listener

	go func() {
		srv := &dns.Server{Addr: ":53", Net: "tcp"}
		err := srv.ListenAndServe()

		if err != nil {
			log.Fatal("Failed to start TCP server.", err.Error())
		}
	}()

	// start listening for OS signals

	sigs := make(chan os.Signal)

	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// quit when a signal is received

	sig := <- sigs

	removeRoutes()
	log.Fatalf("Received signal %s, exiting...", sig.String())
}