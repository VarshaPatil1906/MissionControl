#!/bin/bash

# Wait for services to start
sleep 10

# Submit a mission and capture response
echo "Submitting mission..."
response=$(curl -s -X POST http://localhost:8080/missions \
  -H "Content-Type: application/json" \
  -d '{"command": "Secure the perimeter"}')

# Extract mission ID from JSON response using jq
mission_id=$(echo "$response" | ./jq -r '.mission_id')


echo "Mission submitted! Mission ID: $mission_id"

# Check mission status using the extracted ID
echo "Checking status..."
curl -s http://localhost:8080/missions/$mission_id
echo

# Wait to allow processing (optional)
sleep 20

echo "Test completed."
