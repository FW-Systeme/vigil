#!/bin/bash
curl -sf http://localhost:${1:-8080}/ > /dev/null
