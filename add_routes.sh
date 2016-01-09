#!/bin/bash
echo Starting OpenVPN...
rm -f openvpn_out.txt
( openvpn "UK London.ovpn" 2>&1 1>openvpn_out.txt )&

vpn4=
until [[ ! -z $vpn4 ]]; do
	vpn4=$(cat openvpn_out.txt | grep -Po '(?<=DHCP-serv: )[0-9\.]{4,}')
	sleep 1
done

gwv4=$(ipconfig | awk '/^Ethernet adapter Intel/{s=1} s==1 && /Default Gateway/{g=1} s==1&&g==1&&e!=1&&/[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}/{print $0;e=1}' | grep -Po '[0-9\.]{4,}')
gwv6=$(ipconfig | awk '/^Ethernet adapter Intel/{s=1} s==1 && /Default Gateway/{g=1} s==1&&g==1&&e!=1&&/(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))/{print $0;e=1}' | grep -Po '[a-f0-9:]{4,}')

echo "LAN IPv4 Gateway: $gwv4"
echo "LAN IPv6 Gateway: $gwv6"
echo "VPN IPv4 Address: $vpn4"

while read cidr; do
	echo "Routing $cidr to $vpn4..."
	route add "$cidr" "$vpn4"
done <netflix_v4.txt

while read cidr; do
	echo "Routing $cidr to $gwv6..."
	route add "$cidr" "$gwv6" if 1
done <netflix_v6.txt