# Test for local dev
curl -s -X POST http://127.0.0.1:8080/nucleus.v1.NucleusService/GetStarterImplementation \
  -H "Content-Type: application/json" \
  -d '{"project_id": "proj-123", "requirement_code": "REQ-1.01"}' | jq .

# Test validation (empty fields → INVALID_INPUT)
curl -s -X POST http://127.0.0.1:8080/nucleus.v1.NucleusService/GetStarterImplementation \
  -H "Content-Type: application/json" \
  -d '{}' | jq .