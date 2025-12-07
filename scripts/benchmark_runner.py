#
#    Author: Joyjeet Roy
#
import subprocess
import time
import csv
import requests
import concurrent.futures
import statistics
import os
import yaml
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.dirname(SCRIPT_DIR)

LB_URL = "http://localhost:8080"
CONFIG_FILE = os.path.join(PROJECT_ROOT, "config.yaml")
OUTPUT_FILE = os.path.join(PROJECT_ROOT, "results", "production_env_benchmarks.csv")
ALGORITHMS = ["round-robin", "least-connections", "weighted-round-robin", "least-response-time", "q-learning"]

TOTAL_REQUESTS = 500
CONCURRENCY = 10

def write_config(algo):
    config = {
        "port": 8080,
        "algorithm": algo,
        "health_check_interval": "2s",
        "backends": [
            {"url": "http://localhost:8081", "weight": 1},
            {"url": "http://localhost:8082", "weight": 1},
            {"url": "http://localhost:8083", "weight": 1},
            {"url": "http://localhost:8084", "weight": 1},
            {"url": "http://localhost:8085", "weight": 1},
        ]
    }
    with open(CONFIG_FILE, "w") as f:
        yaml.dump(config, f)

def run_load_test():
    latencies = []
    errors = 0
    start_time = time.time()
    
    with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENCY) as executor:
        futures = [executor.submit(requests.get, LB_URL) for _ in range(TOTAL_REQUESTS)]
        for future in concurrent.futures.as_completed(futures):
            try:
                resp = future.result()
                if resp.status_code != 200:
                    errors += 1
                latencies.append(resp.elapsed.total_seconds() * 1000)
            except Exception:
                errors += 1
    
    duration = time.time() - start_time
    return latencies, errors, duration

def main():
    print("Starting Production Environment Benchmark (All Features Enabled)...")
    
    mock_script = os.path.join(PROJECT_ROOT, "simulation", "mock_servers.py")
    
    print(f"Starting Mock Servers ({mock_script})...")
    mock_process = subprocess.Popen(
        ["python", mock_script],
        stdout=subprocess.DEVNULL, 
        stderr=subprocess.DEVNULL,
        cwd=PROJECT_ROOT
    )
    time.sleep(3) 

    results = []
    
    try:
        for algo in ALGORITHMS:
            print(f"\n---> Testing Algorithm: {algo}")
            
            write_config(algo)
            
            print("Starting Load Balancer...")
            lb_process = subprocess.Popen(
                ["go", "run", "main.go"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                cwd=PROJECT_ROOT 
            )
            time.sleep(5) 
            
            print("Sending traffic...")
            try:
                latencies, errors, duration = run_load_test()
                
                avg_lat = statistics.mean(latencies) if latencies else 0
                p95 = statistics.quantiles(latencies, n=20)[18] if latencies else 0
                throughput = TOTAL_REQUESTS / duration
                
                print(f"Results: Avg={avg_lat:.2f}ms, P95={p95:.2f}ms, Err={errors}, RPS={throughput:.2f}")
                
                results.append({
                    "Algorithm": algo,
                    "Total Requests": TOTAL_REQUESTS,
                    "Concurrency": CONCURRENCY,
                    "Throughput (Req/s)": round(throughput, 2),
                    "Avg Latency (ms)": round(avg_lat, 2),
                    "P95 Latency (ms)": round(p95, 2),
                    "Error Rate (%)": round((errors/TOTAL_REQUESTS)*100, 2)
                })
            finally:
                lb_process.terminate()
                try:
                    lb_process.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    lb_process.kill()
                time.sleep(2)

    finally:
        print("Stopping Mock Servers...")
        mock_process.terminate()
        try:
            mock_process.wait(timeout=2)
        except subprocess.TimeoutExpired:
            mock_process.kill()

    keys = results[0].keys()
    with open(OUTPUT_FILE, "w", newline="") as f:
        dict_writer = csv.DictWriter(f, fieldnames=keys)
        dict_writer.writeheader()
        dict_writer.writerows(results)
        
    print(f"\nDone! Results saved to {os.path.abspath(OUTPUT_FILE)}")

if __name__ == "__main__":
    main()
