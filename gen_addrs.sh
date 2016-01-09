#!/bin/bash

# resolve domains to A and AAAA records

echo -n > addrs_v4.txt
echo -n > addrs_v6.txt

echo Resolving domains...

while read dns; do
	dig +noall +answer a "$dns" | awk '$4~/^A$/{print $5}' >> addrs_v4.txt
	dig +noall +answer aaaa "$dns" | awk '$4~/^AAAA$/{print $5}' >> addrs_v6.txt
done <domains.txt

# remove duplicates before lookup

echo Deduplicating list...

{ rm addrs_v4.txt && sort -n | uniq > addrs_v4.txt; } < addrs_v4.txt
{ rm addrs_v6.txt && sort -n | uniq > addrs_v6.txt; } < addrs_v6.txt

# transfrom address list into a CIDR list

echo Resolving addresses...

echo -n > addrs_v4_tmp.txt
echo -n > addrs_v6_tmp.txt

while read addr; do
	whois "$addr" | awk '/^(CIDR|route):/&&/\//{print $2}' | tr -d ', ' >> addrs_v4_tmp.txt
done <addrs_v4.txt

while read addr; do
	whois "$addr" | awk '/^inet6num:/&&/\//{print $2}' | tr -d ', ' >> addrs_v6_tmp.txt
done <addrs_v6.txt

cat addrs_v4_tmp.txt > addrs_v4.txt
cat addrs_v6_tmp.txt > addrs_v6.txt

rm -f addrs_v4_tmp.txt
rm -f addrs_v6_tmp.txt

echo "94.125.179.8/32" >> addrs_v4.txt

# remove duplicates since Windows will complain otherwise

echo Deduplicating list...

{ rm addrs_v4.txt && sort -n | uniq > addrs_v4.txt; } < addrs_v4.txt
{ rm addrs_v6.txt && sort -n | uniq > addrs_v6.txt; } < addrs_v6.txt