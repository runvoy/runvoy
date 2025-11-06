curl -sS \
        -X GET "http://localhost:${RUNVOY_DEV_SERVER_PORT}/api/v1/executions/{{execution_id}}/logs" \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" | jq