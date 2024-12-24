#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# Base URL
BASE_URL="http://localhost:8080"

# Test user credentials
USERNAME="testuser$(( RANDOM % 100000 ))"
PASSWORD="testpass123"

echo "Starting betting flow tests with user: $USERNAME"

# Setup: Register and login
echo -e "\n1. Setting up test user..."
curl -s -X POST "$BASE_URL/api/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}"

LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
    echo -e "${RED}Failed to get auth token${NC}"
    exit 1
fi

# Show initial balance
echo -e "\nInitial balance:"
curl -s -X GET "$BASE_URL/api/user/balance" \
    -H "Authorization: Bearer $TOKEN"

# Test 1: Betting with insufficient balance
echo -e "\n${GREEN}Test 1: Betting with insufficient balance${NC}"
RESPONSE=$(curl -s -X POST "$BASE_URL/api/bet" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"amount": 1500.0}')
echo "Response: $RESPONSE"
echo "Expected: insufficient balance error"

# Test 2: Try betting with invalid amount
echo -e "\n${GREEN}Test 2: Betting with invalid amount${NC}"
RESPONSE=$(curl -s -X POST "$BASE_URL/api/bet" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"amount": -50.0}')
echo "Response: $RESPONSE"
echo "Expected: $(curl -s -X POST "$BASE_URL/api/bet" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"amount": -50.0}' | jq -r '.error')"

# Test 3: Place valid bet
echo -e "\n${GREEN}Test 3: Place valid bet${NC}"
# Wait for betting phase
echo "Waiting for betting phase..."
while true; do
    GAME_STATE=$(curl -s -X GET "$BASE_URL/api/game/current" \
        -H "Authorization: Bearer $TOKEN" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
    if [ "$GAME_STATE" = "betting" ]; then
        break
    fi
    sleep 0.5
done

RESPONSE=$(curl -s -X POST "$BASE_URL/api/bet" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"amount": 100.0}')
echo "Response: $RESPONSE"
echo "Expected: successful bet"

# Test 4: Try placing second bet in same game
echo -e "\n${GREEN}Test 4: Attempting double bet${NC}"
RESPONSE=$(curl -s -X POST "$BASE_URL/api/bet" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"amount": 50.0}')
echo "Response: $RESPONSE"
echo "Expected: already placed bet error"

# Test 5: Try betting during game progress
echo -e "\n${GREEN}Test 5: Betting during game progress${NC}"
echo "Waiting for game to start..."
while true; do
    GAME_STATE=$(curl -s -X GET "$BASE_URL/api/game/current" \
        -H "Authorization: Bearer $TOKEN" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
    echo "Current game state: $GAME_STATE"
    if [ "$GAME_STATE" = "in_progress" ]; then
        break
    fi
    sleep 0.5
done

echo "Game in progress, attempting bet..."
RESPONSE=$(curl -s -X POST "$BASE_URL/api/bet" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"amount": 50.0}')
echo "Response: $RESPONSE"
echo "Expected: game not in betting phase error"

# Test 6: Verify balance was deducted
echo -e "\n${GREEN}Test 6: Checking balance after bet${NC}"
BALANCE_RESPONSE=$(curl -s -X GET "$BASE_URL/api/user/balance" \
    -H "Authorization: Bearer $TOKEN")
echo "Final balance: $BALANCE_RESPONSE"
echo "Expected: ~900.0 (initial 1000 - bet 100)"

echo -e "\n${GREEN}Testing complete!${NC}" 