#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# Base URL
BASE_URL="http://localhost:8080"

# Check if server is running
echo "Checking if server is running..."
if ! curl -s "$BASE_URL/health" > /dev/null; then
    echo -e "${RED}Error: Server is not running at $BASE_URL${NC}"
    echo "Please start the server first with: go run cmd/server/main.go"
    exit 1
fi

# 1. Admin login
echo -e "\n1. Admin login..."
ADMIN_LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}')
ADMIN_TOKEN=$(echo "$ADMIN_LOGIN_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "$ADMIN_TOKEN" ]; then
    echo -e "${RED}Failed to get admin token. Make sure admin user exists.${NC}"
    exit 1
fi

# 2. Get admin stats
echo -e "\n2. Getting admin statistics..."
STATS_RESPONSE=$(curl -s -X GET "$BASE_URL/api/admin/stats" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
echo "Stats response: $STATS_RESPONSE"

# 3. Get pending withdrawals
echo -e "\n3. Getting pending withdrawals..."
WITHDRAWALS_RESPONSE=$(curl -s -X GET "$BASE_URL/api/admin/withdrawals" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
echo "Pending withdrawals response: $WITHDRAWALS_RESPONSE"

# 4. Process a withdrawal
echo -e "\n4. Processing withdrawal..."
PROCESS_RESPONSE=$(curl -s -X POST "$BASE_URL/api/admin/withdrawals/process" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"withdrawalId":"LATEST_ID","status":"approved"}')
echo "Process response: $PROCESS_RESPONSE"

# 5. Get system settings
echo -e "\n5. Getting system settings..."
SETTINGS_RESPONSE=$(curl -s -X GET "$BASE_URL/api/admin/settings" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
echo "Settings response: $SETTINGS_RESPONSE"

echo -e "\n${GREEN}Admin endpoint tests complete!${NC}" 