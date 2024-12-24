#!/bin/bash

# Generate a random username to avoid conflicts
RANDOM_USER="testuser$RANDOM"
echo "Testing with user: $RANDOM_USER"

# 1. Register
echo -e "\n1. Registering new user..."
REGISTER_RESPONSE=$(curl -s -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$RANDOM_USER\", \"password\": \"testpass123\"}")
echo "Register response: $REGISTER_RESPONSE"

# 2. Login
echo -e "\n2. Logging in..."
LOGIN_RESPONSE=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$RANDOM_USER\", \"password\": \"testpass123\"}")
echo "Login response: $LOGIN_RESPONSE"

# Extract token
TOKEN=$(echo $LOGIN_RESPONSE | grep -o '"token":"[^"]*' | grep -o '[^"]*$')
echo "Token: $TOKEN"

# 3. Check initial balance
echo -e "\n3. Checking initial balance..."
curl -s -X GET http://localhost:8080/api/user/balance \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"

# 4. Place bet
echo -e "\n4. Placing bet..."
curl -s -X POST http://localhost:8080/api/bet \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount": 100}'

# 5. Wait and cashout
echo -e "\n5. Waiting 2 seconds before cashout..."
sleep 2
echo "Cashing out..."
curl -s -X POST http://localhost:8080/api/cashout \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"

# 6. Check final balance
echo -e "\n6. Checking final balance..."
curl -s -X GET http://localhost:8080/api/user/balance \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"

echo -e "\nTest complete!" 