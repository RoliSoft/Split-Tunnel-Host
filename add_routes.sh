#!/bin/bash

if [[ $# -ne 1 ]]; then
	echo usage: add_routes.sh cn; exit 1
fi

if [[ ! -f "pia/$1.ovpn" ]]; then
	echo "pia/$1.ovpn" does not exist; exit 1
fi

# start OpenVPN

echo Starting OpenVPN...

rm -f openvpn_out.txt
( cd pia; openvpn --writepid ../openvpn_pid.txt --config "$1.ovpn" > ../openvpn_out.txt 2>&1 )&

# extract "DHCP-serv" IP from log
# warning: this may be PIA-specific

echo Waiting for gateway IP...

vpn4=
until [[ ! -z $vpn4 ]]; do
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

# extract default gateway for the LAN
# Windows does not support null-routing the way Linux does, so IPv6 addresses need to be routed to the IPv6 default gateway, where they will be dropped

gwv4=$(ipconfig | awk '/^Ethernet adapter Intel/{s=1} s==1 && /Default Gateway/{g=1} s==1&&g==1&&e!=1&&/[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}/{print $0;e=1}' | grep -Po '[0-9\.]{4,}')
gwv6=$(ipconfig | awk '/^Ethernet adapter Intel/{s=1} s==1 && /Default Gateway/{g=1} s==1&&g==1&&e!=1&&/(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))/{print $0;e=1}' | grep -Po '[a-f0-9:]{4,}')

echo "LAN IPv4 Gateway: $gwv4"
echo "LAN IPv6 Gateway: $gwv6"
echo "VPN IPv4 Address: $vpn4"

# set-up IPv4 addresses to route through the VPN

echo Adding IPv4 addresses...

while read cidr; do
	echo "Routing $cidr to $vpn4..."
	route add "$cidr" "$vpn4"
done <addrs_v4.txt

# set-up IPv6 addresses to null-route

echo Adding IPv6 addresses...

while read cidr; do
	echo "Routing $cidr to $gwv6..."
	route add "$cidr" "$gwv6" if 1
done <addrs_v6.txt