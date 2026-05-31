import os
import random
import sys
import time
from datetime import datetime, timezone

from dotenv import load_dotenv
import requests

load_dotenv()

WRITE_API_KEY = os.getenv("WRITE_API_KEY")
if not WRITE_API_KEY:
    print(
        "Error: WRITE_API_KEY is missing from the root .env file. "
        "Please check your configuration."
    )
    sys.exit(1)

# Define both environments clearly
ENV_URLS = {
    "local": "http://localhost:8080/ingest",
    "prod": "http://54.219.97.94/api/ingest"
}

# Automatically use local, unless you type 'prod' after the filename
target_env = "prod" if "prod" in sys.argv else "local"
TARGET_URL = ENV_URLS[target_env]

# Frequency control: 1 second for local testing, 60+ seconds for cloud
DELAY_SECONDS = 15

# The "Hardware" we are simulating
DEVICES = ["SN-ALPHA-01", "SN-BETA-02", "SN-GAMMA-03"]


def generate_mock_data(device_id):
    """Generates realistic random data within your schema.yaml bounds."""
    return {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "serial_number": device_id,
        "temperature": round(random.uniform(35.0, 65.0), 1),
        "humidity": round(random.uniform(30.0, 50.0), 1),
        "bus_voltage": round(random.choice([12.0, 24.0]) + random.uniform(-0.4, 0.4), 2),
    }


def main():
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
                    "X-API-Key": WRITE_API_KEY,
                }

                try:
                    response = requests.post(
                        TARGET_URL, json=payload, headers=headers, timeout=5
                    )

                    if response.status_code in (200, 201, 202):
                        print(
                            f"[SUCCESS] {device_id} -> Temp: {payload['temperature']}C | "
                            f"Volts: {payload['bus_voltage']}V"
                        )
                    else:
                        print(
                            f"[ERROR] {device_id} -> Server rejected. "
                            f"Code {response.status_code}: {response.text}"
                        )

                except requests.exceptions.RequestException as e:
                    print(f"[FAILED] {device_id} -> Connection error: {e}")

            time.sleep(DELAY_SECONDS)

    except KeyboardInterrupt:
        print("\nSimulation powered down.")


if __name__ == "__main__":
    main()
