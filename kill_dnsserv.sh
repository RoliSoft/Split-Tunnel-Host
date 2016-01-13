#!/bin/bash

os=$(uname -o)

if [[ ${os} == "Cygwin" ]]; then
	if [ $(uname -m) == "x86_64" ]; then
		SendCtrlC="./SendCtrlC64"
	else
		SendCtrlC="./SendCtrlC"
	fi
else
	SendCtrlC="kill -INT"
fi

# switch back to DHCP

echo Switching back to DHCP...

if [[ ${os} == "Cygwin" ]]; then
	netsh interface ipv4 set dnsserver "Intel" dhcp | awk '!/^\s*$/{print $0}'
	netsh interface ipv6 set dnsserver "Intel" dhcp | awk '!/^\s*$/{print $0}'

	ipconfig /flushdns | awk '!/(^\s*$|^Windows IP Configuration$)/{print $0}'
else
	cat original_resolv.conf > /etc/resolv.conf

	test -f /etc/init.d/nscd    && /etc/init.d/nscd    restart
	test -f /etc/init.d/dnsmasq && /etc/init.d/dnsmasq restart
	test -f /etc/init.d/named   && /etc/init.d/named   restart
fi

# stop our OpenVPN instance, if running

if [[ -f openvpn_pid.txt ]]; then
	echo Stopping OpenVPN...

	${SendCtrlC} $(cat openvpn_pid.txt | tr -d '\r\n ')
	rm -f openvpn_pid.txt
fi

# stop our DNS server, if running

if [[ -f dnsserv_pid.txt ]]; then
	echo Stopping DNS server...

	${SendCtrlC} $(cat dnsserv_pid.txt | tr -d '\r\n ')
	rm -f dnsserv_pid.txt
fi