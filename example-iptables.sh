#!/bin/bash
# -*- coding: UTF8 -*-
iptables -t nat -A OUTPUT -o lo -j ACCEPT
iptables -t nat -A OUTPUT -d 10.0.0.0/8 -p tcp -m tcp -j REDIRECT --to-ports 10080
iptables -t nat -A OUTPUT -p udp -m udp --dport 53 -j REDIRECT --to-ports 10053
