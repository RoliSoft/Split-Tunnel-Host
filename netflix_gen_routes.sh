#!/bin/bash
wget https://ip-ranges.amazonaws.com/ip-ranges.json -O amazon_ip_ranges.json
grep -Po '(?<=/net/)([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}' netflix-bgp.he.net.html > netflix_v4.txt
grep -Po '(?<=/net/)([a-f0-9]{1,4}\:)[^"]+' netflix-bgp.he.net.html > netflix_v6.txt
grep -Po '([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}' amazon_ip_ranges.json >> netflix_v4.txt
echo 94.125.179.8/32 >> netflix_v4.txt
echo 2a01:578::/32 >> netflix_v6.txt