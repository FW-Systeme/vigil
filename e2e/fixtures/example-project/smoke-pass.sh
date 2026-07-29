#!/bin/bash
curl -sf http://localhost:${1:-3000}/ > /dev/null
