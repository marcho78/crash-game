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

# Generate random username
USERNAME="testuser$(( RANDOM % 100000 ))"
PASSWORD="testpass123"

# 1. Register and login
echo -e "\n1. Setting up test user..."
REGISTER_RESPONSE=$(curl -s -X POST "$BASE_URL/api/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
echo "Register response: $REGISTER_RESPONSE"

LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

# 2. Get latest game ID and hash
echo -e "\n2. Getting latest game info..."
GAME_INFO=$(curl -s -X GET "$BASE_URL/api/game/current" \
    -H "Authorization: Bearer $TOKEN")
GAME_ID=$(echo "$GAME_INFO" | grep -o '"gameId":"[^"]*' | cut -d'"' -f4)
GAME_HASH=$(echo "$GAME_INFO" | grep -o '"hash":"[^"]*' | cut -d'"' -f4)

echo "Latest Game ID: $GAME_ID"
echo "Game Hash: $GAME_HASH"

# 3. Create a withdrawal request and get its ID
echo -e "\n3. Creating withdrawal request..."
WITHDRAW_RESPONSE=$(curl -s -X POST "$BASE_URL/api/user/withdraw" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"amount":50}')
WITHDRAWAL_ID=$(echo "$WITHDRAW_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

echo "Withdrawal ID: $WITHDRAWAL_ID"

# 4. Verify game with actual ID and hash
echo -e "\n4. Testing game verification..."
VERIFY_RESPONSE=$(curl -s -X POST "$BASE_URL/api/game/verify" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"gameId\":\"$GAME_ID\",\"hash\":\"$GAME_HASH\"}")
echo "Verify response: $VERIFY_RESPONSE"

# Check if withdrawal endpoint exists in server routes
echo -e "\n5. Getting user balance..."
BALANCE_RESPONSE=$(curl -s -X GET "$BASE_URL/api/user/balance" \
    -H "Authorization: Bearer $TOKEN")
echo "Balance response: $BALANCE_RESPONSE"

# 6. Place bet
echo -e "\n6. Placing bet..."
BET_RESPONSE=$(curl -s -X POST "$BASE_URL/api/bet" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"amount":50}')
echo "Bet response: $BET_RESPONSE"

# 7. Get game history
echo -e "\n7. Getting game history..."
HISTORY_RESPONSE=$(curl -s -X GET "$BASE_URL/api/game/history" \
    -H "Authorization: Bearer $TOKEN")
echo "History response: $HISTORY_RESPONSE"

echo -e "\n${GREEN}Tests complete!${NC}" 