#!/bin/bash

BASE_URL="http://localhost:8080"
CONCURRENT=3
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

echo "[${LOG_TIME}] [INFO] [Single] Mission submitted: $single_mission_id"
echo "[${LOG_TIME}] [INFO] [Single] Waiting for completion..."
elapsed=0
while [[ $elapsed -lt $WAIT_TIMEOUT ]]; do
  single_status=$(curl -s "$BASE_URL/missions/$single_mission_id" | ./jq.exe -r '.status')
  echo "[${LOG_TIME}] [INFO] [Single] Status: $single_status (Elapsed: ${elapsed}s)"
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

echo "[${LOG_TIME}] [INFO] [Concurrency] Submitting $CONCURRENT missions in parallel..."
for i in $(seq 1 $CONCURRENT); do
  (
    res=$(curl -s -X POST $BASE_URL/missions \
      -H "Content-Type: application/json" \
      -d "{\"command\": \"Concurrent mission $i\"}")
    id=$(echo "$res" | ./jq.exe -r '.mission_id')
    echo $id > "mission_id_$i.txt"
    echo "[${LOG_TIME}] [INFO] [Concurrency] Mission $i submitted: $id"
  ) &
done

wait

for i in $(seq 1 $CONCURRENT); do
  id=$(cat mission_id_$i.txt)
  mission_ids+=("$id")
  rm mission_id_$i.txt
done

# Wait until all missions are COMPLETED/FAILED or timeout occurs
for idx in "${!mission_ids[@]}"; do
  id="${mission_ids[$idx]}"
  elapsed=0
  status=""
  echo "[${LOG_TIME}] [INFO] [Concurrency] Waiting for mission $((idx+1)) ($id)..."
  while [[ $elapsed -lt $WAIT_TIMEOUT ]]; do
    status=$(curl -s "$BASE_URL/missions/$id" | ./jq.exe -r '.status')
    echo "[${LOG_TIME}] [INFO] [Concurrency] Mission $((idx+1)) Status: $status (Elapsed: ${elapsed}s)"
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

echo "[${LOG_TIME}] [INFO] [Auth] Submitting test missions with different tokens..."
for tname in "EXPIRED_TOKEN" "INVALID_TOKEN" "VALID_TOKEN"; do
  TOKEN=${!tname}
  res=$(curl -s -X POST $BASE_URL/missions \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"command": "Token test mission"}')
  mission_id=$(echo "$res" | ./jq.exe -r '.mission_id')
  echo "[${LOG_TIME}] [INFO] [Auth] Token: $tname, Mission ID: $mission_id"
  if [[ "$mission_id" == "null" ]]; then
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
echo "[${LOG_TIME}] [INFO] ==== All Tests Completed. OVERALL: $overall_status ===="
