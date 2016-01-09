#!/bin/bash

# resolve domains to A and AAAA records

echo -n > addrs_v4.txt
echo -n > addrs_v6.txt

echo Resolving domains...

while read dns; do
	dig +noall +answer a "$dns" | awk '$4~/^A$/{print $5}' >> addrs_v4.txt
	dig +noall +answer aaaa "$dns" | awk '$4~/^AAAA$/{print $5}' >> addrs_v6.txt
done <domains.txt

# remove duplicates since Windows will complain otherwise

echo Deduplicating list...

{ rm addrs_v4.txt && sort -n | uniq > addrs_v4.txt; } < addrs_v4.txt
{ rm addrs_v6.txt && sort -n | uniq > addrs_v6.txt; } < addrs_v6.txt

# transfrom address list into a CIDR list
# this is problematic, normally you would do a whois for the IP and use the returned range,
# but in most cases this would return a CIDR which covers all the servers in use by the
# cloud provider, and this might not be what we want
# the current implementation just uses /24 for IPv4 and /64 for IPv6

echo Processing list...

{ rm addrs_v4.txt && awk -F'.' '{print $1"."$2"."$3".0/24"}' > addrs_v4.txt; } < addrs_v4.txt
{ rm addrs_v6.txt && awk -F'.' '{print $0"/64"}' > addrs_v6.txt; } < addrs_v6.txt

echo "94.125.179.8/32" >> addrs_v4.txt