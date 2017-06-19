#!/bin/bash
# Run with arguments you want to pass to test.
# Example: ./test.sh -run TestTwoSingleNodeClusters
export PATH=/usr/local/go/bin:$PATH
set -xe
cd tests
sudo -E `which go` test -v "$@"
