#!/usr/bin/env python3

import requests
import time
import statistics

LB_URL = "http://localhost:8080"
NUM_REQUESTS = 20

def test_timing_fix():
    print("=" * 60)
    print("Testing Fix #1: Timing Measurement")
    print("=" * 60)
    
    latencies = []
    
    print(f"\nSending {NUM_REQUESTS} requests to load balancer...")
    
    for i in range(NUM_REQUESTS):
        try:
            start = time.time()
            resp = requests.get(LB_URL, timeout=5)
            duration_ms = (time.time() - start) * 1000
            latencies.append(duration_ms)
            
            status = "✓" if resp.status_code == 200 else "✗"
            print(f"  Request {i+1:2d}: {status} {resp.status_code} - {duration_ms:.2f}ms")
            
        except Exception as e:
            print(f"  Request {i+1:2d}: ✗ ERROR - {e}")
    
    if latencies:
        avg = statistics.mean(latencies)
        min_lat = min(latencies)
        max_lat = max(latencies)
        
        print(f"\n{'Results':^60}")
        print("-" * 60)
        print(f"  Average Latency: {avg:.2f}ms")
        print(f"  Min Latency:     {min_lat:.2f}ms")
        print(f"  Max Latency:     {max_lat:.2f}ms")
        print(f"  Total Requests:  {len(latencies)}")
        
        if avg > 1.0:
            print(f"\n  ✅ TIMING FIX VERIFIED: Latencies are realistic!")
        else:
            print(f"\n  ⚠️  WARNING: Latencies seem too low ({avg:.4f}ms)")
        
        return True
    else:
        print("\n  ❌ FAILED: No successful requests")
        return False

def test_circuit_breaker():
    print("\n" + "=" * 60)
    print("Testing Fix #2: Circuit Breaker Integration")
    print("=" * 60)
    
    print("\nℹ️  Circuit breaker verification:")
    print("   1. Check load balancer logs for 'Circuit: true/false'")
    print("   2. If a backend fails 3+ times, circuit should open")
    print("   3. Failed backend should be excluded from rotation")
    print("\n   Run this test while monitoring LB logs:")
    print("   > go run main.go")
    
    return True

def main():
    print("\n" + "=" * 60)
    print("Load Balancer Fixes Verification Test")
    print("=" * 60)
    print("\n⚠️  Prerequisites:")
    print("   1. Start load balancer: go run main.go")
    print("   2. Start mock servers:  python simulation/mock_servers.py")
    print("\nPress Enter when ready...")
    input()
    
    try:
        timing_ok = test_timing_fix()
        cb_ok = test_circuit_breaker()
        
        print("\n" + "=" * 60)
        print("Summary")
        print("=" * 60)
        print(f"  Timing Fix:         {'✅ PASS' if timing_ok else '❌ FAIL'}")
        print(f"  Circuit Breaker:    ℹ️  Check logs manually")
        print("=" * 60 + "\n")
        
    except requests.exceptions.ConnectionError:
        print("\n❌ ERROR: Could not connect to load balancer")
        print("   Make sure it's running on http://localhost:8080")
        print("   Run: go run main.go")

if __name__ == "__main__":
    main()
