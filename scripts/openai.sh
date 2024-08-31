#!/bin/bash

addr=${1:-"http://localhost:8001"}

curl "http://localhost:8001/v1/chat/completions" \
  -H 'accept: application/json' \
  -d'{"messages":[{"role":"user","content":"hi"},"model":"gpt-3.5-turbo","temperature":0.5,"presence_penalty":0,"frequency_penalty":0,"top_p":1}'