#!/usr/bin/env bash
set -euo pipefail

echo "=== merge test suite ==="

# Clean previous outputs
rm -rf proto-download proto-bundle proto-split

echo ""
echo "--- Go vet ---"
go vet ./...

echo ""
echo "--- Unit tests: proto (parse/bundle/split) ---"
go test ./proto/ -v -count=1

echo ""
echo "--- Unit tests: descriptor (describe/log/interceptor) ---"
go test ./descriptor/ -v -count=1

echo ""
echo "--- Unit tests: transform (add field/method, merge, rename, remove) ---"
go test ./transform/ -v -count=1

echo ""
echo "--- Unit tests: walk (walk + transform pipeline) ---"
go test ./walk/ -v -count=1

echo ""
echo "--- Unit tests: build (full pipeline) ---"
go test ./build/ -v -count=1

echo ""
echo "--- Unit tests: server (gRPC service) ---"
go test ./server/ -v -count=1

echo ""
echo "--- gRPC smoke test ---"
if command -v grpcurl &>/dev/null; then
  GRPC_TEST_PORT=$((10000 + RANDOM % 50000))
  # Kill anything on that port just in case
  lsof -ti :${GRPC_TEST_PORT} | xargs kill 2>/dev/null || true
  go run ./cmd/ --serve --addr 127.0.0.1:${GRPC_TEST_PORT} &
  SERVER_PID=$!
  sleep 3

  echo "  listing services..."
  grpcurl -plaintext 127.0.0.1:${GRPC_TEST_PORT} list | grep merge.Merger
  echo "  calling Describe..."
  RESULT=$(grpcurl -plaintext -d '{"file": {"name": "smoke.proto", "package": "smoke", "message_type": [{"name": "Msg"}]}}' 127.0.0.1:${GRPC_TEST_PORT} merge.Merger/Describe)
  echo "$RESULT" | grep -q "smoke.proto" && echo "  Describe: OK" || (echo "  Describe: FAIL"; kill $SERVER_PID; exit 1)

  echo "  calling Transform (add uuid)..."
  RESULT=$(grpcurl -plaintext -d '{
    "file": {"name": "t.proto", "package": "t", "syntax": "proto3",
      "message_type": [{"name": "A", "field": [{"name": "x", "number": 1, "type": 9, "label": 1}]}]},
    "commands": [{"add_field": {"field_name": "uuid", "field_type": 9}}]
  }' 127.0.0.1:${GRPC_TEST_PORT} merge.Merger/Transform)
  echo "$RESULT" | grep -q '"uuid"' && echo "  Transform: OK" || (echo "  Transform: FAIL"; kill $SERVER_PID; exit 1)

  kill $SERVER_PID 2>/dev/null
  wait $SERVER_PID 2>/dev/null || true
  echo "  gRPC smoke test passed"
else
  echo "  grpcurl not installed, skipping gRPC smoke test"
fi

echo ""
echo "--- Integration: scan accretional org ---"
go run ./cmd/ --org accretional

echo ""
echo "--- Validate: proto-download/ has .proto files ---"
DOWNLOAD_COUNT=$(find proto-download -name '*.proto' 2>/dev/null | wc -l | tr -d ' ')
echo "proto-download: ${DOWNLOAD_COUNT} files"
if [ "$DOWNLOAD_COUNT" -eq 0 ]; then
  echo "FAIL: no files in proto-download/"
  exit 1
fi

echo ""
echo "--- Validate: proto-bundle/bundle.proto exists ---"
if [ ! -f proto-bundle/bundle.proto ]; then
  echo "FAIL: proto-bundle/bundle.proto missing"
  exit 1
fi
BUNDLE_LINES=$(wc -l < proto-bundle/bundle.proto | tr -d ' ')
echo "proto-bundle/bundle.proto: ${BUNDLE_LINES} lines"
if [ "$BUNDLE_LINES" -lt 5 ]; then
  echo "FAIL: bundle.proto too small"
  exit 1
fi

echo ""
echo "--- Validate: proto-split/ has per-package dirs and files ---"
SPLIT_COUNT=$(find proto-split -name '*.proto' 2>/dev/null | wc -l | tr -d ' ')
echo "proto-split: ${SPLIT_COUNT} files"
if [ "$SPLIT_COUNT" -eq 0 ]; then
  echo "FAIL: no files in proto-split/"
  exit 1
fi
SPLIT_DIRS=$(find proto-split -type d | wc -l | tr -d ' ')
echo "proto-split: ${SPLIT_DIRS} directories"

echo ""
echo "--- Validate: proto-split has service files ---"
SVC_COUNT=$(find proto-split -name '*_service.proto' 2>/dev/null | wc -l | tr -d ' ')
echo "proto-split: ${SVC_COUNT} service files"

echo ""
echo "=== All checks passed ==="
