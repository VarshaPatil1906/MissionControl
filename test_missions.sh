#!/bin/bash

BASE_URL="http://localhost:8080"
CONCURRENT=15
LOG_TIME="$(date +"%Y-%m-%d %H:%M:%S")"
WAIT_TIMEOUT=60     # seconds to wait for each mission (adjust as needed)
WAIT_INTERVAL=2     # polling interval in seconds

print_header() {
  echo "[${LOG_TIME}] [INFO] ==============================="
  echo "[${LOG_TIME}] [INFO]   FINAL MISSION TEST SUMMARY   "
  echo "[${LOG_TIME}] [INFO] ==============================="
}

print_header

# Test 1: Single Mission Submission 
single_test_status="FAIL"
single_response=$(curl -s -X POST $BASE_URL/missions \
    -H "Content-Type: application/json" \
    -d '{"command": "Secure the perimeter"}')
single_mission_id=$(echo "$single_response" | ./jq.exe -r '.mission_id')

# Wait until mission is completed or timeout occurs
elapsed=0
while [[ $elapsed -lt $WAIT_TIMEOUT ]]; do
  single_status=$(curl -s "$BASE_URL/missions/$single_mission_id" | ./jq.exe -r '.status')
  if [[ "$single_status" == "COMPLETED" || "$single_status" == "FAILED" ]]; then
    break
  fi
  sleep $WAIT_INTERVAL
  elapsed=$((elapsed + WAIT_INTERVAL))
done

if [[ "$single_status" == "COMPLETED" ]]; then
    single_test_status="PASS"
fi

echo "[${LOG_TIME}] [INFO] Test 1: Single Mission Flow ........ $single_test_status"

# Test 2: Concurrency (Parallel) 
concurrent_test_status="PASS"
concurrent_completed=0
concurrent_failed=0
declare -a mission_ids

for i in $(seq 1 $CONCURRENT); do
  (
    res=$(curl -s -X POST $BASE_URL/missions \
      -H "Content-Type: application/json" \
      -d "{\"command\": \"Concurrent mission $i\"}")
    id=$(echo "$res" | ./jq.exe -r '.mission_id')
    echo $id > "mission_id_$i.txt"
  ) &
done

wait

for i in $(seq 1 $CONCURRENT); do
  id=$(cat mission_id_$i.txt)
  mission_ids+=("$id")
  rm mission_id_$i.txt
done

# Wait until all missions are COMPLETED/FAILED or timeout occurs
for id in "${mission_ids[@]}"; do
  elapsed=0
  status=""
  while [[ $elapsed -lt $WAIT_TIMEOUT ]]; do
    status=$(curl -s "$BASE_URL/missions/$id" | ./jq.exe -r '.status')
    if [[ "$status" == "COMPLETED" || "$status" == "FAILED" ]]; then
      break
    fi
    sleep $WAIT_INTERVAL
    elapsed=$((elapsed + WAIT_INTERVAL))
  done

  if [[ "$status" == "COMPLETED" ]]; then
    ((concurrent_completed++))
  else
    ((concurrent_failed++))
  fi
done

if [[ $concurrent_failed -gt 0 ]]; then
    concurrent_test_status="FAIL"
fi

echo "[${LOG_TIME}] [INFO] Test 2: Concurrency ($CONCURRENT Missions) ... $concurrent_test_status"
echo "[${LOG_TIME}] [INFO]   (COMPLETED: $concurrent_completed, FAILED: $concurrent_failed)"

# Test 3: Authentication & Token Rotation 
auth_test_status="PASS"
EXPIRED_TOKEN="expiredTokenExample"
INVALID_TOKEN="invalidTokenExample"
VALID_TOKEN="validCurrentTokenExample"

TOKENS=(
  "$EXPIRED_TOKEN"
  "$INVALID_TOKEN"
  "$VALID_TOKEN"
)

for TOKEN in "${TOKENS[@]}"; do
  res=$(curl -s -X POST $BASE_URL/missions \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"command": "Token test mission"}')
  if [[ $(echo "$res" | ./jq.exe -r '.mission_id') == "null" ]]; then
    auth_test_status="FAIL"
  fi
done

echo "[${LOG_TIME}] [INFO] Test 3: Token Rotation ............ $auth_test_status"

# Final Summary
overall_status="PASS"
if [[ $single_test_status != "PASS" || $concurrent_test_status != "PASS" || $auth_test_status != "PASS" ]]; then
  overall_status="FAIL"
fi

echo ""
