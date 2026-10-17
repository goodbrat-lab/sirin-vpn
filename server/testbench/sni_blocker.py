#!/usr/bin/env python3
# sni_blocker.py — блокирует TLS ClientHello с указанным SNI
from netfilterqueue import NetfilterQueue
from scapy.all import *
import sys

BLOCKED_SNI = b"sun8-21.userapi.com"

def process_packet(pkt):
    try:
        payload = pkt.get_payload()
        ip = IP(payload)
        if ip.haslayer(TCP) and ip[TCP].dport == 443:
            tcp_payload = bytes(ip[TCP].payload)
            # Простой поиск SNI в payload (можно улучшить парсингом)
            if BLOCKED_SNI in tcp_payload:
                print(f"Blocking SNI: {BLOCKED_SNI.decode()}", flush=True)
                pkt.drop()
                return
    except Exception as e:
        pass
    pkt.accept()

nfqueue = NetfilterQueue()
nfqueue.bind(1, process_packet) # очередь №1
try:
    print("sni_blocker started listening on queue 1...", flush=True)
    nfqueue.run()
except KeyboardInterrupt:
    pass
