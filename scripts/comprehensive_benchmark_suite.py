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
import json
from collections import defaultdict

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.dirname(SCRIPT_DIR)
MOCK_CONFIG_PATH = os.path.join(PROJECT_ROOT, "simulation", "mock_config.json")
MOCK_SCRIPT_PATH = os.path.join(PROJECT_ROOT, "simulation", "mock_servers.py")
LB_URL = "http://localhost:8080"
CONFIG_FILE = os.path.join(PROJECT_ROOT, "config.yaml")
FINAL_OUTPUT_FILE = os.path.join(PROJECT_ROOT, "results", "comprehensive_suite_results.csv")

ALGORITHMS = ["round-robin", "least-connections", "weighted-round-robin", "least-response-time", "q-learning"]


PORTS = [8081, 8082, 8083, 8084, 8085]
REQUESTS_PER_ALGO = 200 
CONCURRENCY = 10

SCENARIOS = {
    "Baseline": [
        {"port": p, "latency": 0.010, "jitter": 0.002, "fail_rate": 0.0, "name": f"Base_{i}"} for i, p in enumerate(PORTS)
    ],
    "The_Trap": [
        {"port": 8081, "latency": 0.001, "jitter": 0.001, "fail_rate": 0.50, "name": "TRAP"},
        {"port": 8082, "latency": 0.050, "jitter": 0.005, "fail_rate": 0.00, "name": "SAFE_1"},
        {"port": 8083, "latency": 0.050, "jitter": 0.005, "fail_rate": 0.00, "name": "SAFE_2"},
        {"port": 8084, "latency": 0.050, "jitter": 0.005, "fail_rate": 0.00, "name": "SAFE_3"},
        {"port": 8085, "latency": 0.200, "jitter": 0.020, "fail_rate": 0.00, "name": "SLOW"},
    ],
    "High_Jitter": [
        {"port": p, "latency": 0.020, "jitter": 0.200, "fail_rate": 0.0, "name": f"Jitter_{i}"} for i, p in enumerate(PORTS)
    ],
    "One_Dead": [
        {"port": 8081, "latency": 0.010, "jitter": 0.001, "fail_rate": 0.00, "name": "OK_1"},
        {"port": 8082, "latency": 0.010, "jitter": 0.001, "fail_rate": 0.00, "name": "OK_2"},
        {"port": 8083, "latency": 0.010, "jitter": 0.001, "fail_rate": 1.00, "name": "DEAD"},
        {"port": 8084, "latency": 0.010, "jitter": 0.001, "fail_rate": 0.00, "name": "OK_3"},
        {"port": 8085, "latency": 0.010, "jitter": 0.001, "fail_rate": 0.00, "name": "OK_4"},
    ],
    "Sloth_Invasion": [
        {"port": 8081, "latency": 0.005, "jitter": 0.001, "fail_rate": 0.00, "name": "HERO"},
        {"port": 8082, "latency": 0.300, "jitter": 0.010, "fail_rate": 0.00, "name": "SLOTH"},
        {"port": 8083, "latency": 0.300, "jitter": 0.010, "fail_rate": 0.00, "name": "SLOTH"},
        {"port": 8084, "latency": 0.300, "jitter": 0.010, "fail_rate": 0.00, "name": "SLOTH"},
        {"port": 8085, "latency": 0.300, "jitter": 0.010, "fail_rate": 0.00, "name": "SLOTH"},
    ],
    "Linear_Decay": [
        {"port": PORTS[i], "latency": 0.010 * (i+1), "jitter": 0.002, "fail_rate": 0.0, "name": f"Grad_{i}"} for i in range(5)
    ],
    "Spiky": [
        {"port": p, "latency": 0.010, "jitter": 0.500, "fail_rate": 0.0, "name": f"Spike_{i}"} for i, p in enumerate(PORTS)
    ],
    "Heavy_Load": [
        {"port": p, "latency": 0.500, "jitter": 0.050, "fail_rate": 0.0, "name": f"Heavy_{i}"} for i, p in enumerate(PORTS)
    ],
    "Reliability": [
        {"port": p, "latency": 0.050, "jitter": 0.000, "fail_rate": 0.0, "name": f"Reliable_{i}"} for i, p in enumerate(PORTS)
    ],
    "Chaos": [
        {"port": 8081, "latency": 0.01, "jitter": 0.20, "fail_rate": 0.20, "name": "CHAOS_1"},
        {"port": 8082, "latency": 0.05, "jitter": 0.05, "fail_rate": 0.05, "name": "CHAOS_2"},
        {"port": 8083, "latency": 0.10, "jitter": 0.10, "fail_rate": 0.00, "name": "CHAOS_3"},
        {"port": 8084, "latency": 0.01, "jitter": 0.00, "fail_rate": 0.50, "name": "CHAOS_4"},
        {"port": 8085, "latency": 0.20, "jitter": 0.00, "fail_rate": 0.00, "name": "CHAOS_5"},
    ],
}

def write_lb_config(algo):
    config = {
        "port": 8080,
        "algorithm": algo,
        "health_check_interval": "1s",
        "backends": [{"url": f"http://localhost:{p}", "weight": 1} for p in PORTS]
    }
    with open(CONFIG_FILE, "w") as f:
        yaml.dump(config, f)

def run_test():
    latencies = []
    errors = 0
    start = time.time()
    with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENCY) as executor:
        futures = [executor.submit(requests.get, LB_URL) for _ in range(REQUESTS_PER_ALGO)]
        for future in concurrent.futures.as_completed(futures):
            try:
                resp = future.result()
                if resp.status_code != 200: errors += 1
                latencies.append(resp.elapsed.total_seconds() * 1000)
            except:
                errors += 1
    duration = time.time() - start
    return latencies, errors, duration

def main():
    scenario_names = list(SCENARIOS.keys())
    total_scenarios = len(scenario_names)
    print(f"Starting Comprehensive Benchmark Suite ({total_scenarios} Scenarios x {len(ALGORITHMS)} Algos)...")
    
    overall_results = []
    
    
    for i, scenario_name in enumerate(scenario_names):
        print(f"\n\n=== Scenario {i+1}/{total_scenarios}: {scenario_name} ===")
        
        
        servers = SCENARIOS[scenario_name]
        with open(MOCK_CONFIG_PATH, "w") as f:
            json.dump(servers, f, indent=2)
            
        
        print("  Running Mock Servers...")
        
        
        
        
        time.sleep(1)
        
        
        
        mock_proc = subprocess.Popen(["python", MOCK_SCRIPT_PATH], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=PROJECT_ROOT)
        time.sleep(2)
        
        for algo in ALGORITHMS:
            print(f"  Testing {algo}...", end="", flush=True)
            write_lb_config(algo)
            
            
            lb_proc = subprocess.Popen(["go", "run", "main.go"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=PROJECT_ROOT)
            time.sleep(3)
            
            try:
                lats, errs, dur = run_test()
                rps = REQUESTS_PER_ALGO / dur
                avg_lat = statistics.mean(lats) if lats else 0
                err_rate = (errs / REQUESTS_PER_ALGO) * 100
                
                result = {
                    "Scenario": scenario_name,
                    "Algorithm": algo,
                    "RPS": round(rps, 2),
                    "Latency": round(avg_lat, 2),
                    "Error_Rate": round(err_rate, 2)
                }
                overall_results.append(result)
                print(f" Done. RPS={result['RPS']}, Lat={result['Latency']}ms")
                
            finally:
                lb_proc.terminate()
                try: lb_proc.wait(timeout=2)
                except: lb_proc.kill()
        
        
        mock_proc.terminate()
        try: mock_proc.wait(timeout=2)
        except: mock_proc.kill()

    
    print("\n\n=== AGGREGATING RESULTS ===")
    
    headers = ["Algorithm", "Avg_RPS", "Avg_Latency", "Avg_Error_Rate"]
    algo_stats = defaultdict(lambda: {"rps": [], "lat": [], "err": []})
    
    for r in overall_results:
        a = r["Algorithm"]
        algo_stats[a]["rps"].append(r["RPS"])
        algo_stats[a]["lat"].append(r["Latency"])
        algo_stats[a]["err"].append(r["Error_Rate"])
        
    final_summary = []
    for algo in ALGORITHMS:
        stats = algo_stats[algo]
        final_summary.append({
            "Algorithm": algo,
            "Avg_RPS": round(statistics.mean(stats["rps"]), 2),
            "Avg_Latency": round(statistics.mean(stats["lat"]), 2),
            "Avg_Error_Rate": round(statistics.mean(stats["err"]), 2)
        })
        
    final_summary.sort(key=lambda x: x["Avg_RPS"], reverse=True)
    
    with open(FINAL_OUTPUT_FILE, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=headers)
        writer.writeheader()
        writer.writerows(final_summary)
        
    
    raw_path = os.path.join(PROJECT_ROOT, "results", "comprehensive_suite_raw.csv")
    with open(raw_path, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=overall_results[0].keys())
        writer.writeheader()
        writer.writerows(overall_results)
        
    print(f"\nFinal Summary saved to {FINAL_OUTPUT_FILE}")
    print(f"Raw Data saved to {raw_path}")
    
    print("\nAlgorithm | Avg RPS | Avg Latency | Avg Error %")
    print("-" * 50)
    for r in final_summary:
        print(f"{r['Algorithm']:<10} | {r['Avg_RPS']:<7} | {r['Avg_Latency']:<11} | {r['Avg_Error_Rate']}")

if __name__ == "__main__":
    main()
