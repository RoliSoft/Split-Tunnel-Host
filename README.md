# Host-based Split Tunneling

The purpose of this project is to mess with the Windows routing table in order to tunnel specific 3rd-party hosts and services selectively to a VPN connection. It was born out of the need of me not wanting to route all of my Internet traffic through a commercial VPN just because of a single service.

## Implementation

The `gen_addrs.sh` script looks up the A and AAAA records for the domains listed in `domains.txt`. This list of IPs is then looked up using whois, and the CIDR notation for the netblock they are in is stored in `addrs_v4.txt` and `addrs_v6.txt`, depending on their IP version.

The `add_routes.sh` script starts OpenVPN with the specified configuration, waits until the connection succeeds and extracts the gateway IP. (In case of PIA, this is the same as the address of the DHCP server, so this address will be used instead, since it's readily available in the log. You may need to tweak this for other providers.) After this, the Windows routing table will be modified (using the `route` command) to route the previously extracted IP ranges through the VPN.

Since most VPN providers don't support IPv6 at this time, the extracted IPv6 ranges will be null-routed. Windows does not support null-routing the same way Linux does, as such the addresses will instead be set up to be routed to the local gateway, where they will be dropped.

The `del_routes.sh` script stops the running OpenVPN instance and removes all the set routes. Since a pid-file is used, this script will only stop the instance specifically started by `add_routes.sh`, and not do `killall openvpn`, so it is safe to use with multiple active VPN connections.

The `pia` folder contains modified `.ovpn` files for PIA. The modifications that were done to the originally distributed configuration files (untouched available in `pia/others`) were `route-noexec` to ignore the routes pushed by the server and `auth-user-pass login.conf` which instructs OpenVPN to load the username and password from the `login.conf` file.

## Side Effects

While this technique works great otherwise, you'll need to take great care when compiling the list of domains. In case of Netflix, for example, you will end up null-routing the whole Amazon IPv6 range and routing their IPv4 ranges through the VPN. This is not a big deal, since you will still be able to access Amazon and miscellaneous EC2 servers, but you'll go through the VPN.

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