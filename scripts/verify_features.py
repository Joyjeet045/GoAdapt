import subprocess
import time
import requests
import sys
import os
import signal

def run_verification():
    print(">>> Starting E2E Feature Verification")
    
    
    SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
    PROJECT_ROOT = os.path.dirname(SCRIPT_DIR)
    
    MOCK_SCRIPT = os.path.join(PROJECT_ROOT, "simulation", "mock_servers.py")
    LB_EXE = os.path.join(PROJECT_ROOT, "lb.exe")

    print(f">>> Starting Mock Servers from {MOCK_SCRIPT}...")
    mock_server_process = subprocess.Popen(
        [sys.executable, MOCK_SCRIPT],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        cwd=PROJECT_ROOT
    )
    time.sleep(2)

    print(f">>> Starting Load Balancer from {LB_EXE}...")
    if not os.path.exists(LB_EXE):
        print(f"ERROR: Could not find {LB_EXE}. Please run 'go build -o lb.exe main.go' in the project root.")
        return

    lb_process = subprocess.Popen(
        [LB_EXE],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        cwd=PROJECT_ROOT
    )
    time.sleep(3)

    failures = []

    try:
        try:
            resp = requests.get("http://localhost:8080/healthz")
            if resp.status_code == 200 and resp.text == "ok":
                print("PASS: /healthz endpoint")
            else:
                failures.append(f"FAIL: /healthz returned {resp.status_code} {resp.text}")
        except Exception as e:
            failures.append(f"FAIL: /healthz exception: {e}")

        try:
            resp = requests.get("http://localhost:8080/")
            
            if resp.status_code == 200:
                print("PASS: Basic routing")
            else:
                failures.append(f"FAIL: Basic routing status {resp.status_code}")

            headers = resp.headers
            if "X-Request-ID" in headers:
                print(f"PASS: X-Request-ID found ({headers['X-Request-ID']})")
            else:
                failures.append("FAIL: X-Request-ID missing")

            if "Strict-Transport-Security" in headers:
                print("PASS: Security Headers (HSTS) found")
            else:
                failures.append("FAIL: Security Headers missing")
                
        except Exception as e:
            failures.append(f"FAIL: Basic Request exception: {e}")

        try:
            headers = {"Accept-Encoding": "gzip"}
            resp = requests.get("http://localhost:8080/", headers=headers)
            if "gzip" in resp.headers.get("Content-Encoding", ""):
                print("PASS: Gzip Compression working")
            else:
                failures.append(f"FAIL: Gzip not applied. Content-Encoding: {resp.headers.get('Content-Encoding')}")
        except Exception as e:
            failures.append(f"FAIL: Gzip exception: {e}")

    finally:
        print("\n>>> LB Output Log:")
        try:
            lb_process.terminate()
            outs, errs = lb_process.communicate(timeout=5)
            print(outs.decode('utf-8', errors='ignore'))
            print(errs.decode('utf-8', errors='ignore'))
        except Exception as e:
            print(f"Could not read logs: {e}")

        print("\n>>> Cleaning up...")
        lb_process.terminate()
        mock_server_process.terminate()
        try:
            subprocess.run(["taskkill", "/F", "/T", "/PID", str(lb_process.pid)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            subprocess.run(["taskkill", "/F", "/T", "/PID", str(mock_server_process.pid)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        except:
            pass

    print("\n>>> SUMMARY ======================")
    if not failures:
        print("ALL TESTS PASSED")
    else:
        print(f"{len(failures)} TESTS FAILED")
        for f in failures:
            print(f"- {f}")

if __name__ == "__main__":
    run_verification()
