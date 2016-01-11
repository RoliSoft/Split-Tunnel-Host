#!/bin/bash

if [ $(uname -m) == "x86_64" ]; then
	SendCtrlC="./SendCtrlC64"
else
	SendCtrlC="./SendCtrlC"
fi

# switch back to DHCP

echo Switching back to DHCP...

netsh interface ipv4 set dnsserver "Intel" dhcp | awk '!/^\s*$/{print $0}'
netsh interface ipv6 set dnsserver "Intel" dhcp | awk '!/^\s*$/{print $0}'
ipconfig /flushdns | awk '!/(^\s*$|^Windows IP Configuration$)/{print $0}'

# stop our OpenVPN instance, if running

if [[ -f openvpn_pid.txt ]]; then
	echo Stopping OpenVPN...
	
	$SendCtrlC $(cat openvpn_pid.txt | tr -d '\r\n ')
	rm -f openvpn_pid.txt
fi

# stop our DNS server, if running

if [[ -f dnsserv_pid.txt ]]; then
	echo Stopping DNS server...
	
	$SendCtrlC $(cat dnsserv_pid.txt | tr -d '\r\n ')
	rm -f dnsserv_pid.txt
fi