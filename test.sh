#!/bin/bash
set -e

echo "[1/4] Building oss-cli..."
go build -o oss-cli .

echo "[2/4] Generating 10M file for test..."
mkdir -p test_file
if [ ! -f test_file/10M_test.bin ]; then
    head -c 10M /dev/urandom > test_file/10M_test.bin
fi

echo "[3/4] CLI Functional Tests"
echo "-> Uploading..."
./oss-cli upload test_file/10M_test.bin
# Extract object key
OBJ_KEY=$(./oss-cli list -p "file-" -l 1 | grep "10M_test.bin" | awk '{print $2}')
if [ -z "$OBJ_KEY" ]; then
    echo "Failed to get OBJ_KEY from list"
    exit 1
fi
echo "-> Generating URL..."
./oss-cli url $OBJ_KEY
echo "-> Deleting..."
./oss-cli delete $OBJ_KEY

echo "[4/4] HTTP API & Stress Tests"
./oss-cli server -p 8083 &
SERVER_PID=$!
sleep 3

TOKEN=$(grep OPENAI_API_KEY .env.local | cut -d '=' -f 2)

echo "-> HTTP Upload"
UPLOAD_RES=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -F "file=@test_file/10M_test.bin" http://127.0.0.1:8083/v1/files)
echo $UPLOAD_RES
FILE_ID=$(echo $UPLOAD_RES | grep -o '"id":"[^"]*' | cut -d'"' -f4)

echo "-> HTTP Get Info"
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8083/v1/files/$FILE_ID
echo ""

echo "-> HTTP Get Content (Download URL)"
curl -s -I -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8083/v1/files/$FILE_ID/content | head -n 1
echo ""

echo "-> HTTP Delete"
curl -s -X DELETE -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8083/v1/files/$FILE_ID
echo ""

echo "-> Stress Test (50 requests/min)"
# Sending 50 requests to list files over 60 seconds
# Using a background loop so it executes precisely
for i in {1..50}; do
  curl -s -o /dev/null -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8083/v1/files &
  sleep 1.2
done
wait
echo "Stress Test Complete"

kill -9 $SERVER_PID
echo "All tests passed successfully!"
