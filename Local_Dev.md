# 1. Set your API key
export ANTHROPIC_API_KEY=$(cat ./.anthropic.key)

# 2. Build and run the server
make build
make run

# 3. In another terminal, test the endpoint
curl -s -X POST http://127.0.0.1:8080/nucleus.v1.NucleusService/GetStarterImplementation \
  -H "Content-Type: application/json" \
  -d '{"project_id": "proj-123", "requirement_code": "REQ-1.01"}' | jq .

curl -s -X POST http://127.0.0.1:8080/nucleus.v1.NucleusService/GetStarterImplementation \
  -H "Content-Type: application/json" \
  -d '{}' | jq .

  curl -X POST http://127.0.0.1:8080/nucleus.v1.NucleusService/GetStarterImplementation \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "test-001",
    "requirement_code": "REQ-123"
  }' | jq .