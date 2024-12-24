#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# Base URL
BASE_URL="http://localhost:8080"

# Check if server is running first
echo "Checking if server is running..."
if ! curl -s "$BASE_URL/health" > /dev/null; then
    echo -e "${RED}Error: Server is not running at $BASE_URL${NC}"
    echo "Please start the server first with: go run cmd/server/main.go"
    exit 1
fi

# Generate random username
USERNAME="testuser$(( RANDOM % 100000 ))"
PASSWORD="testpass123"

echo "Starting game flow test with user: $USERNAME"

# 1. Register
echo -e "\n1. Registering new user..."
REGISTER_RESPONSE=$(curl -s -X POST "$BASE_URL/api/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
echo "Register response: $REGISTER_RESPONSE"

# 2. Login
echo -e "\n2. Logging in..."
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
echo "Login response: $LOGIN_RESPONSE"

# Extract token with better error handling
TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
    echo "Failed to get token. Full response:"
    echo "$LOGIN_RESPONSE"
    exit 1
fi

# 3. Get initial balance
echo "3. Initial balance"
echo "Balance: >>> Calling GET /user/balance"
BALANCE_RESPONSE=$(curl -s -X GET "$BASE_URL/api/user/balance" \
    -H "Authorization: Bearer $TOKEN")
echo "Response: $BALANCE_RESPONSE"
echo "---"
echo

# 4. Place bet - wait for betting phase
echo "4. Place bet"
echo "Waiting for betting phase..."
while true; do
    GAME_STATE=$(curl -s -X GET "$BASE_URL/api/game/current" \
        -H "Authorization: Bearer $TOKEN" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
    if [ "$GAME_STATE" = "betting" ]; then
        break
    fi
    sleep 0.5
done

echo "Game is accepting bets, placing bet..."
BET_RESPONSE=$(curl -s -X POST "$BASE_URL/api/bet" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"amount\":100}")
echo "Response: $BET_RESPONSE"
echo "---"
echo

# 5. Wait and attempt cashout
echo "5. Waiting for game to start..."
while true; do
    GAME_STATE=$(curl -s -X GET "$BASE_URL/api/game/current" \
        -H "Authorization: Bearer $TOKEN" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
    if [ "$GAME_STATE" = "in_progress" ]; then
        break
    fi
    sleep 0.5
done

echo "Game in progress, attempting cashout..."
echo "Cashout response: >>> Calling POST /cashout"
CASHOUT_RESPONSE=$(curl -s -X POST "$BASE_URL/api/cashout" \
    -H "Authorization: Bearer $TOKEN")
echo "Response: $CASHOUT_RESPONSE"
echo "---"
echo

# 6. Get final balance
echo "6. Final balance"
echo "Final balance: >>> Calling GET /user/balance"
FINAL_BALANCE_RESPONSE=$(curl -s -X GET "$BASE_URL/api/user/balance" \
    -H "Authorization: Bearer $TOKEN")
echo "Response: $FINAL_BALANCE_RESPONSE"
echo "---"
echo

echo "Test complete!" 