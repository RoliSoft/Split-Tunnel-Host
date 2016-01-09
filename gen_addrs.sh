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