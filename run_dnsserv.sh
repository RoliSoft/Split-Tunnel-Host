#!/bin/bash

if [[ $# -ne 1 ]]; then
	echo usage: add_routes.sh cn; exit 1
fi

if [[ ! -f "pia/$1.ovpn" ]]; then
	echo "pia/$1.ovpn" does not exist; exit 1
fi

if [[ ! -f dnsserv.exe ]]; then
	echo Compiling DNS server...
	go build dnsserv.go

	if [[ ! -f dnsserv.exe ]]; then
		echo Failed to compile DNS server!; exit 1
	fi
fi

# start OpenVPN

echo Starting OpenVPN...

rm -f openvpn_out.txt
( cd pia; openvpn --writepid ../openvpn_pid.txt --config "$1.ovpn" > ../openvpn_out.txt 2>&1 )&

# extract "DHCP-serv" IP from log
# warning: this may be PIA-specific

echo Waiting for gateway IP...

vpn4=
until [[ ! -z ${vpn4} ]]; do
	if [[ -f openvpn_out.txt ]]; then
		vpn4=$(cat openvpn_out.txt | grep -Po '(?<=DHCP-serv: )[0-9\.]{4,}')
	fi
	sleep 1
	if [[ ! -f openvpn_pid.txt ]]; then
		if [[ -f openvpn_out.txt ]]; then
			echo OpenVPN failed to run:
			cat openvpn_out.txt
			exit 1
		else
			echo OpenVPN failed to run.; exit 1
		fi
	fi
done

echo "VPN IPv4 Address: $vpn4"

# start DNS server

echo Starting DNS server...

rm -f dnsserv_out.txt
./dnsserv -r "$vpn4" > dnsserv_out.txt 2>&1 &
echo $! > dnsserv_pid.txt

sleep 1

# switch to DNS local server

echo Switching to DNS server...

netsh interface ipv4 set dnsserver "Intel" static 127.0.0.1 primary | awk '!/^\s*$/{print $0}'
netsh interface ipv6 set dnsserver "Intel" static ::1 primary | awk '!/^\s*$/{print $0}'
ipconfig /flushdns | awk '!/(^\s*$|^Windows IP Configuration$)/{print $0}'