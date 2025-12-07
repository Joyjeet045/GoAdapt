#
#    Author: Joyjeet Roy
#
import http.server
import socketserver
import time
import random
import threading
import sys
import json
import os

DEFAULT_SERVERS = [
    {"port": 8081, "latency": 0.005, "jitter": 0.001, "fail_rate": 0.00, "name": "GOLDEN"},
    {"port": 8082, "latency": 0.050, "jitter": 0.005, "fail_rate": 0.00, "name": "RELIABLE"},
    {"port": 8083, "latency": 0.001, "jitter": 0.001, "fail_rate": 0.50, "name": "TRAP"},
    {"port": 8084, "latency": 0.200, "jitter": 0.020, "fail_rate": 0.00, "name": "SLOTH"},
    {"port": 8085, "latency": 0.150, "jitter": 0.100, "fail_rate": 0.10, "name": "CHAOS"},
]

SERVERS = DEFAULT_SERVERS
CONFIG_PATH = os.path.join(os.path.dirname(__file__), "mock_config.json")

if os.path.exists(CONFIG_PATH):
    try:
        with open(CONFIG_PATH, "r") as f:
            SERVERS = json.load(f)
            print(f"Loaded config from {CONFIG_PATH}")
    except Exception as e:
        print(f"Failed to load config: {e}")

class MockHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        server_conf = self.server.config
        
        if random.random() < server_conf["fail_rate"]:
            self.send_response(500)
            self.end_headers()
            self.wfile.write(b"Internal Server Error")
            return

        delay = server_conf["latency"] + random.uniform(-server_conf["jitter"], server_conf["jitter"])
        if delay < 0: delay = 0
        time.sleep(delay)
        
        self.send_response(200)
        self.send_header('Content-type', 'text/plain')
        self.end_headers()
        self.wfile.write(f"Response from {server_conf['name']} on port {server_conf['port']}".encode())
    
    def log_message(self, format, *args):
        pass

class ThreadingSimpleServer(socketserver.ThreadingMixIn, socketserver.TCPServer):
    pass

def run_server(conf):
    port = conf["port"]
    handler = MockHandler
    with ThreadingSimpleServer(("", port), handler) as httpd:
        httpd.config = conf
        print(f"Starting {conf['name']} server on port {port}")
        httpd.serve_forever()

if __name__ == "__main__":
    threads = []
    print("Starting mock backend servers...")
    for conf in SERVERS:
        t = threading.Thread(target=run_server, args=(conf,), daemon=True)
        t.start()
        threads.append(t)
    
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("Stopping servers...")
