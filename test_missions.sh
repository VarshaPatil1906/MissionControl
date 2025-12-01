#!/bin/bash

# ----------------- Config -----------------
# Which commander to test (1, 2, or 3)
COMMANDER=1

BASE_URL="http://localhost:808${COMMANDER}"   # commander1=8081, commander2=8082, commander3=8083
CONCURRENT=3
WAIT_TIMEOUT=60       # seconds to wait for each mission
WAIT_INTERVAL=2       # polling interval in seconds
jq_bin="./jq.exe"     # path/name of jq

ts() {
  date +"%Y-%m-%d %H:%M:%S"
}

print_header() {
  echo "[$(ts)] [INFO] ==============================="
  echo "[$(ts)] [INFO]   FINAL MISSION TEST SUMMARY   "
  echo "[$(ts)] [INFO] ==============================="
  echo "[$(ts)] [INFO] Testing commander on $BASE_URL"
}

print_header

############################################
# Test 1: Single Mission Submission
############################################
single_test_status="FAIL"

single_response=$(curl -s -X POST "$BASE_URL/missions" \
  -H "Content-Type: application/json" \
  -d '{"payload": "Secure the perimeter", "target_soldier": "soldier1"}')

single_mission_id=$(echo "$single_response" | "$jq_bin" -r '.mission_id')

echo "[$(ts)] [INFO] [Single] Mission submitted: $single_mission_id"
echo "[$(ts)] [INFO] [Single] Waiting for completion..."
elapsed=0
single_status="UNKNOWN"

while [[ $elapsed -lt $WAIT_TIMEOUT ]]; do
  details=$(curl -s "$BASE_URL/missions/$single_mission_id")
  single_status=$(echo "$details"        | "$jq_bin" -r '.status')
  single_soldier=$(echo "$details"       | "$jq_bin" -r '.assigned_soldier')
  single_cmd=$(echo "$details"           | "$jq_bin" -r '.commander_name')
  echo "[$(ts)] [INFO] [Single] Status: $single_status, Soldier: $single_soldier, Commander: $single_cmd (Elapsed: ${elapsed}s)"
  if [[ "$single_status" == "COMPLETED" || "$single_status" == "FAILED" ]]; then
    break
  fi
  sleep "$WAIT_INTERVAL"
  elapsed=$((elapsed + WAIT_INTERVAL))
done

if [[ "$single_status" == "COMPLETED" ]]; then
  single_test_status="PASS"
fi

echo "[$(ts)] [INFO] Test 1: Single Mission Flow ........ $single_test_status"

############################################
# Test 2: Concurrency (Parallel)
############################################
concurrent_test_status="PASS"
concurrent_completed=0
concurrent_failed=0
declare -a mission_ids

echo "[$(ts)] [INFO] [Concurrency] Submitting $CONCURRENT missions in parallel..."
for i in $(seq 1 "$CONCURRENT"); do
  (
    res=$(curl -s -X POST "$BASE_URL/missions" \
      -H "Content-Type: application/json" \
      -d "{\"payload\": \"Concurrent mission $i\", \"target_soldier\": \"soldier1\"}")
    id=$(echo "$res" | "$jq_bin" -r '.mission_id')
    echo "$id" > "mission_id_$i.txt"
    echo "[$(ts)] [INFO] [Concurrency] Mission $i submitted: $id"
  ) &
done

wait

for i in $(seq 1 "$CONCURRENT"); do
  id=$(cat "mission_id_$i.txt")
  mission_ids+=("$id")
  rm "mission_id_$i.txt"
done

for idx in "${!mission_ids[@]}"; do
  id="${mission_ids[$idx]}"
  elapsed=0
  status="UNKNOWN"
  echo "[$(ts)] [INFO] [Concurrency] Waiting for mission $((idx+1)) ($id)..."

  while [[ $elapsed -lt $WAIT_TIMEOUT ]]; do
    details=$(curl -s "$BASE_URL/missions/$id")
    status=$(echo "$details"      | "$jq_bin" -r '.status')
    soldier=$(echo "$details"     | "$jq_bin" -r '.assigned_soldier')
    cmd=$(echo "$details"         | "$jq_bin" -r '.commander_name')
    echo "[$(ts)] [INFO] [Concurrency] Mission $((idx+1)) Status: $status, Soldier: $soldier, Commander: $cmd (Elapsed: ${elapsed}s)"
    if [[ "$status" == "COMPLETED" || "$status" == "FAILED" ]]; then
      break
    fi
    sleep "$WAIT_INTERVAL"
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

echo "[$(ts)] [INFO] Test 2: Concurrency ($CONCURRENT Missions) ... $concurrent_test_status"
echo "[$(ts)] [INFO]   (COMPLETED: $concurrent_completed, FAILED: $concurrent_failed)"

############################################
# Test 3: Authentication & Token Rotation
############################################
auth_test_status="PASS"

echo "[$(ts)] [INFO] [Auth] Submitting 3 missions to exercise token rotation..."
for i in 1 2 3; do
  res=$(curl -s -X POST "$BASE_URL/missions" \
    -H "Content-Type: application/json" \
    -d "{\"payload\": \"Auth test mission $i\", \"target_soldier\": \"soldier1\"}")
  mission_id=$(echo "$res" | "$jq_bin" -r '.mission_id')
  echo "[$(ts)] [INFO] [Auth] Mission $i ID: $mission_id"
  if [[ "$mission_id" == "null" || -z "$mission_id" ]]; then
    auth_test_status="FAIL"
  fi
done

echo "[$(ts)] [INFO] Test 3: Token Rotation (soldier/commander) ... $auth_test_status"

############################################
# Final Summary
############################################
overall_status="PASS"
if [[ "$single_test_status" != "PASS" || "$concurrent_test_status" != "PASS" || "$auth_test_status" != "PASS" ]]; then
  overall_status="FAIL"
fi

echo ""
echo "[$(ts)] [INFO] ==== All Tests Completed. OVERALL: $overall_status ===="
