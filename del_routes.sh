#!/bin/bash

if [ $(uname -m) == "x86_64" ]; then
	SendCtrlC="./SendCtrlC64"
else
	SendCtrlC="./SendCtrlC"
fi

# stop our OpenVPN instance, if running

if [[ -f openvpn_pid.txt ]]; then
	echo Stopping OpenVPN...
	
	$SendCtrlC $(cat openvpn_pid.txt | tr -d '\r\n ')
	rm -f openvpn_pid.txt
fi

# remove IPv4 address routes

echo Removing IPv4 addresses...

while read cidr; do
	route delete "$cidr"
done <addrs_v4.txt

# remove IPv6 address null-routes

echo Removing IPv6 addresses...

while read cidr; do
	route delete "$cidr"
done <addrs_v6.txt