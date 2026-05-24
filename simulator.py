import requests
import time
import random
from datetime import datetime, timezone

# --- CONFIGURATION ---
# Toggle these variables when you move from Local to Cloud
TARGET_URL = "http://localhost:8080/ingest"
API_KEY = "write_key_change_me"  # <-- PASTE YOUR WRITE KEY HERE

# Frequency control: 1 second for local testing, 60+ seconds for cloud
DELAY_SECONDS = 15 

# The "Hardware" we are simulating
DEVICES = ["SN-ALPHA-01", "SN-BETA-02", "SN-GAMMA-03"]

def generate_mock_data(device_id):
    """Generates realistic random data within your schema.yaml bounds."""
    return {
        # ISO 8601 format, strictly in UTC
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "serial_number": device_id,
        
        # Simulating a device running warm
        "temperature": round(random.uniform(35.0, 65.0), 1),
        
        # Standard environmental humidity
        "humidity": round(random.uniform(30.0, 50.0), 1),
        
        # Simulating minor voltage fluctuations on 12V and 24V buses
        "bus_voltage": round(random.choice([12.0, 24.0]) + random.uniform(-0.4, 0.4), 2)
    }

print(f"Starting telemetry simulator for {len(DEVICES)} devices...")
print(f"Target: {TARGET_URL}")
print(f"Interval: {DELAY_SECONDS} seconds")
print("Press Ctrl+C to stop.")
print("-" * 40)

try:
    while True:
        for device_id in DEVICES:
            payload = generate_mock_data(device_id)
            headers = {
                "Content-Type": "application/json",
                "X-API-Key": API_KEY
            }
            
            try:
                # Fire the packet at the Go backend
                response = requests.post(TARGET_URL, json=payload, headers=headers, timeout=5)
                
                if response.status_code in (200, 201, 202):
                    print(f"[SUCCESS] {device_id} -> Temp: {payload['temperature']}C | Volts: {payload['bus_voltage']}V")
                else:
                    print(f"[ERROR] {device_id} -> Server rejected. Code {response.status_code}: {response.text}")
                    
            except requests.exceptions.RequestException as e:
                print(f"[FAILED] {device_id} -> Connection error: {e}")
                
        # Wait before the next polling cycle
        time.sleep(DELAY_SECONDS)

except KeyboardInterrupt:
    print("\nSimulation powered down.")