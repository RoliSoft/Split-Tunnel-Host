#!/bin/bash

# stop our OpenVPN instance, if running

if [[ -f openvpn_pid.txt ]]; then
	echo Stopping OpenVPN...
	
	/bin/kill -s INT -f $(cat openvpn_pid.txt | tr -d '\r\n ') 2>&1 1>/dev/null
	if [ $? -ne 0 ]; then
		echo Failed to stop OpenVPN on PID $(cat openvpn_pid.txt)
	fi
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