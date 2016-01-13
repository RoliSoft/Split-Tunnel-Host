# Host-based Split Tunneling

The purpose of this project is to mess with the Windows routing table in order to tunnel specific 3rd-party hosts and services selectively to a VPN connection. It was born out of the need of me not wanting to route all of my Internet traffic through a commercial VPN just because of a single service.

# Method 1 -- Static Netblock Routing

This method makes a DNS request for each listed domain and then looks up the netblocks the IP belong to via whois. These netblocks are then routed through the VPN in case of IPv4, and null-routed in case of IPv6. This works, but comes with some side-effects, such as routing irrelevant services on the same netblock via the VPN or failure to compile an exhaustive list of netblocks due to heavy DNS load-balancing. For more information, see [Side Effects](#side-effects).

See the [second method](#method-2----dynamic-per-ip-routing) for split-tunneling services such as Netflix without side-effects.

## Implementation

The `gen_addrs.sh` script looks up the A and AAAA records for the domains listed in `domains.txt`. This list of IPs is then looked up using whois, and the CIDR notation for the netblock they are in is stored in `addrs_v4.txt` and `addrs_v6.txt`, depending on their IP version.

The `add_routes.sh` script starts OpenVPN with the specified configuration, waits until the connection succeeds and extracts the gateway IP. (In case of PIA, this is the same as the address of the DHCP server, so this address will be used instead, since it's readily available in the log. You may need to tweak this for other providers.) After this, the Windows routing table will be modified (using the `route` command) to route the previously extracted IP ranges through the VPN.

Since most VPN providers don't support IPv6 at this time, the extracted IPv6 ranges will be null-routed. Windows does not support null-routing the same way Linux does, as such the addresses will instead be set up to be routed to the local gateway, where they will be dropped.

The `del_routes.sh` script stops the running OpenVPN instance and removes all the set routes. Since a pid-file is used, this script will only stop the instance specifically started by `add_routes.sh`, and not do `killall openvpn`, so it is safe to use with multiple active VPN connections.

The `pia` folder contains modified `.ovpn` files for PIA. The modifications that were done to the originally distributed configuration files (untouched available in `pia/others`) were `route-noexec` to ignore the routes pushed by the server and `auth-user-pass login.conf` which instructs OpenVPN to load the username and password from the `login.conf` file.

## Side Effects

While this technique works great otherwise, you'll need to take great care when compiling the list of domains. In case of Netflix, for example, you will end up null-routing the whole Amazon IPv6 range and routing their IPv4 ranges through the VPN. This is not a big deal, since you will still be able to access Amazon and miscellaneous EC2 servers, but you'll go through the VPN. Another issue is that if the service is heavily load-balanced on the DNS side, you might miss some addresses/netblocks, and since DNS answers expire, your browser might get a fresh batch of addresses outside of the tunneled netblocks at a later request.

If you know the addresses or netblocks you want to tunnel, you can create and fill `addrs_v4.txt` manually, since the `domains.txt` file is not used by `add_routes.sh`. You may also need to create an empty `addrs_v6.txt` file, even if you don't plan on null-routing any IPv6 ranges.

## Dependencies

Since these are Bash scripts, Cygwin is required to run them. The following extra Cygwin packages need to be installed: `whois`, `ipcalc`, `dig`, `gawk`.

Additionally, `openvpn.exe` has to be in your `%PATH%`. A Cygwin-specific distribution is not required, just run the standard Windows installer for the Community Edition.

## Usage

1. Create `login.conf` in the `pia` folder, with the first line being your username, and the second being the password.
2. Fill the `domains.txt` file with the list of domains you would like to split-tunnel inclusively.
3. Run `gen_addrs.sh` in order to generate `addrs_v4.txt` and `addrs_v6.txt` from `domains.txt` **OR** manually create `addrs_v4.txt` and `addrs_v6.txt` with IPv4 CIDR notations of the netblocks you want to route selectively, and IPv6 netblocks you want to null-route.
4. Edit `add_routes.sh` and replace `Ethernet adapter Intel` with the name of your actual LAN adapter, as seen in `ipconfig`.
5. Run `add_routes.sh us` (or any file name as the parameter from the `pia` folder) to start the VPN and add the routes.
6. When finished, run `del_routes.sh` to stop the VPN and remove the routes. If not run, the routes will clear on the next Windows restart anyways, since they are intentionally not set as persistent.

# Method 2 -- Dynamic Per-IP Routing

This method is a more active approach. It implements a DNS server in _Go_ which handles specified domains and forwards irrelevant requests to the default nameservers. When a request of type `A` is seen for the specified domain or any of its subdomains, all of the IP addresses in the reply are added to the routing table. For requests of type `AAAA` and the specified domain, an empty response is sent back, unless tunneling is requested, in which case they will get the same treatment as the records of type `A`. The routed addresses are automatically removed upon stopping the DNS server.

## Implementation

The `run_dnsserv.sh` script starts OpenVPN with the specified configuration, waits until the connection succeeds and extracts the gateway IP. After this, it starts the DNS server in the background as well, and then switches the DNS configuration for the specified network adapter to it.

The `kill_dnsserv.sh` script stops the running OpenVPN and DNS server instances, if there are any, switches the DNS back to DHCP for the specified network adapter and flushes DNS cache. Since a pid-file is used, this script will only stop the instance specifically started by `run_dnsserv.sh`, and not do `killall openvpn; killall dnsserv`, so it is safe to use with multiple active VPN connections.

The `dnsserv.go` file is the DNS server itself. It uses the [miekg/dns](http://github.com/miekg/dns) DNS library both to act as a server and a client for forwarding. The `handleRequest()` function decides the path to take for every DNS request, this is what you'll have to modify with the domains you want to split-tunnel dynamically. For requests of type `A`, every IP in the response will be added to the routing table to use the gateway IP specified in command line argument `-r`. This is automatically specified when started by the `run_dnsserv.sh` script. For requests of type `AAAA`, an empty response will be sent back, unless an IPv6 gateway address is specified via the argument `-r6`, in which case they will get the same treatment as records of type `A`. (Initially IPv6 hijacking was done with an `NXDOMAIN` error in the reply, but Chrome would then just show an error message stating `DNS_PROBE_FINISHED_NXDOMAIN`, even though it sent an `A` request which was then answered properly.) For domains not looked after in the function, the requests will be proxied to the actual nameservers and back without any tampering.

The `SendCtrlC.exe` and `SendCtrlC64.exe` are used to gracefully end both the DNS server and OpenVPN by sending a Ctrl+C message to the processes. Windows does not implement signaling the way Linux does, so even though both apps handle `SIGINT`/Ctrl+C, `taskkill /im` or `/bin/kill -s INT -f` (in Cygwin) do not shut the processes down in a way which would trigger their signal handler. These binaries were compiled from a modified source of the [SendSignal](http://www.latenighthacking.com/projects/2003/sendsignal/) project, which originally sends a Ctrl+Break signal.

## Dependencies

Since these are Bash scripts, Cygwin is required to run them. You will also need to install the `gawk` Cygwin package.

Additionally, `go.exe` and `openvpn.exe` have to be in your `%PATH%`. The standard Windows installers for both Golang and OpenVPN will work, no special Cygwin-specific build is required.

## Usage

1. Create `login.conf` in the `pia` folder, with the first line being your username, and the second being the password.
2. Edit `run_dnsserv.sh` and `kill_dnsserv.sh` and replace the adapter name to yours in the `netsh` calls.
3. Edit `dnsserv.go` and update the filtering logic in `isTargetZone()` as needed.
4. Fetch the dependencies and build the DNS server:

        go get github.com/miekg/dns
        go build -ldflags '-s' dnsserv.go

5. Run `run_dnsserv.sh us` (or any file name as the parameter from the `pia` folder) to start the VPN and the DNS server.
6. When finished, run `kill_dnsserv.sh` to stop the perviously started servers and remove the routes. If not run, since the DNS configuration is set manually by the start script to `localhost`, you may not have DNS connectivity on the next Windows start. To fix this, just run `kill_dnsserv.sh`, as it will restore the network adapter to use the DNS settings specified by DHCP.