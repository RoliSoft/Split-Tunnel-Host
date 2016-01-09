#!/bin/bash
echo Stopping OpenVPN...
kill $(ps a | grep -i 'openvpn/bin/openvpn' | awk '{print $1}')
#taskkill /f /im openvpn.exe

while read cidr; do
	echo "Removing $cidr..."
	route delete "$cidr"
done <netflix_v4.txt

while read cidr; do
	echo "Removing $cidr..."
	route delete "$cidr"
done <netflix_v6.txt