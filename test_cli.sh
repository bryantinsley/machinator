#!/bin/bash
export MACHINATOR_DIR=$(pwd)/tmp_machinator_home
echo "--- Test 1: Project 999 ---"
./machinator_bin/machinator --project=999 --headless
RET=$?
if [ $RET -ne 0 ]; then
  echo "Exited with error $RET (Expected)"
else
  echo "Exited with success (Unexpected)"
fi

echo "--- Test 2: Project 1 ---"
./machinator_bin/machinator --project=1 --headless --once
RET=$?
if [ $RET -ne 0 ]; then
    echo "Exited with error $RET (Check logs)"
else
    echo "Exited with success (Expected)"
fi
