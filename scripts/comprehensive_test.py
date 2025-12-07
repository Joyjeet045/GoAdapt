#
#    Author: Joyjeet Roy
#
import subprocess
import time
import requests
import concurrent.futures
import statistics
import os
import yaml
import csv
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.dirname(SCRIPT_DIR)

LB_URL = "http://localhost:8080"
RELOAD_URL = "http://localhost:8080/reload"
CONFIG_FILE = os.path.join(PROJECT_ROOT, "config.yaml")
OUTPUT_FILE = os.path.join(PROJECT_ROOT, "results", "comprehensive_results.csv")

def write_config(algo):
    config = {
        "port": 8080,
        "algorithm": algo,
        "health_check_interval": "1s",
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

def test_rate_limiting():
    print("Testing Rate Limiter...")
    success = 0
    blocked = 0
    total = 1200
    start = time.time()
    
    with concurrent.futures.ThreadPoolExecutor(max_workers=50) as executor:
        futures = [executor.submit(requests.get, LB_URL) for _ in range(total)]
        for future in concurrent.futures.as_completed(futures):
            try:
                if future.result().status_code == 429:
                    blocked += 1
                else:
                    success += 1
            except Exception as e:
                print(f"Request failed: {e}")
                pass
                
    print(f"Rate Limit Results: Success={success}, Blocked={blocked}")
    return blocked

def measure_performance(name):
    latencies = []
    errors = 0
    blocked = 0
    total = 500
    start = time.time()
    
    with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
        futures = [executor.submit(requests.get, LB_URL) for _ in range(total)]
        for future in concurrent.futures.as_completed(futures):
            try:
                resp = future.result()
                if resp.status_code == 429:
                    blocked += 1
                    errors += 1
                elif resp.status_code != 200:
                    errors += 1
                latencies.append(resp.elapsed.total_seconds() * 1000)
            except:
                errors += 1
                
    duration = time.time() - start
    avg = statistics.mean(latencies) if latencies else 0
    p95 = statistics.quantiles(latencies, n=20)[18] if latencies else 0
    rps = total / duration
    
    return {
        "Test": name,
        "RPS": round(rps, 2),
        "Avg_Latency": round(avg, 2),
        "P95_Latency": round(p95, 2),
        "Error_Rate": round((errors/total)*100, 2),
        "Blocked_Requests": blocked
    }

def main():
    results = []
    
    mock_script = os.path.join(PROJECT_ROOT, "simulation", "mock_servers.py")
    mock_proc = subprocess.Popen(
        ["python", mock_script], 
        stdout=subprocess.DEVNULL, 
        stderr=subprocess.DEVNULL,
        cwd=PROJECT_ROOT
    )
    time.sleep(2)
    
    try:
        write_config("round-robin")
        lb_proc = subprocess.Popen(
            ["go", "run", "main.go"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            cwd=PROJECT_ROOT
        )
        time.sleep(5)
        
        limit_blocked_count = test_rate_limiting()
        results.append({
            "Test": "Rate_Limiter_Check",
            "RPS": 0, "Avg_Latency": 0, "P95_Latency": 0,
            "Error_Rate": 0,
            "Blocked_Requests": limit_blocked_count
        })
        time.sleep(2)
        
        perf = measure_performance("Performance_RoundRobin")
        results.append(perf)
        print(f"RoundRobin: {perf}")
        
        write_config("least-connections")
        requests.get(RELOAD_URL)
        time.sleep(2)
        
        perf = measure_performance("Performance_After_HotReload_LeastConn")
        results.append(perf)
        print(f"HotReload_LeastConn: {perf}")

        
    finally:
        if 'lb_proc' in locals():
            lb_proc.terminate()
            try: lb_proc.wait(timeout=2)
            except: lb_proc.kill()
        mock_proc.terminate()
        try: mock_proc.wait(timeout=2)
        except: mock_proc.kill()
        
    keys = results[0].keys()
    with open(OUTPUT_FILE, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=keys)
        writer.writeheader()
        writer.writerows(results)
    
    print(f"Results saved to {OUTPUT_FILE}")

if __name__ == "__main__":
    main()
